# Local speech stack

Whisper (STT) and Kokoro (TTS) as independent containers, per handoff §8A. Both bind
to loopback only and need no API keys or paid services.

Docker on this host requires the `docker` group. If your shell predates that group,
prefix commands with `sg docker -c '...'` or start a new login session.

## Services

| Service | Image | Host endpoint | Bound to |
|---|---|---|---|
| Whisper STT | `ghcr.io/ggml-org/whisper.cpp:main-vulkan` | `http://127.0.0.1:8770` | loopback only |
| Kokoro TTS | `jplings-kokoro-onnx:local` (built from `./kokoro-onnx`) | `http://127.0.0.1:8771` | loopback only |

Whisper gets `/dev/dri` passed through and logs its Vulkan device on start. Kokoro runs
on CPU.

## Start, stop, clean up

```bash
cd infra
sg docker -c 'docker compose up -d'          # build + start both
sg docker -c 'docker compose ps'             # status
sg docker -c 'docker compose logs -f whisper'
sg docker -c 'docker compose stop'           # stop, keep containers
sg docker -c 'docker compose down'           # remove containers and network
```

Switch models or thread counts through the environment:

```bash
# Whisper model size
WHISPER_MODEL=ggml-medium.bin sg docker -c 'cd infra && WHISPER_MODEL=ggml-medium.bin docker compose up -d --force-recreate whisper'

# Kokoro quantisation and ONNX intra-op threads
KOKORO_MODEL=kokoro-v1.0.int8.onnx ORT_INTRA_OP_THREADS=6 \
  sg docker -c 'cd infra && KOKORO_MODEL=kokoro-v1.0.int8.onnx ORT_INTRA_OP_THREADS=6 docker compose up -d --force-recreate kokoro'
```

Defaults: `ggml-small.bin`, `kokoro-v1.0.onnx` (FP32), 4 threads, language `ja`.

## Models

Weights live in `./models` and are gitignored; they are not committed. Whisper models
are bind-mounted read-only at `/models`. Kokoro's voices and ONNX graphs come from the
same directory.

```bash
mkdir -p models
# Whisper (ggml)
curl -L -o models/ggml-small.bin  https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin
curl -L -o models/ggml-medium.bin https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin
# Kokoro
base=https://github.com/thewh1teagle/kokoro-onnx/releases/download/model-files-v1.0
curl -L -o models/kokoro-v1.0.onnx      "$base/kokoro-v1.0.onnx"
curl -L -o models/kokoro-v1.0.int8.onnx "$base/kokoro-v1.0.int8.onnx"
curl -L -o models/voices-v1.0.bin        "$base/voices-v1.0.bin"
```

## Endpoint contract

These are the shapes the Go `internal/speech` clients are written against.

### STT — `POST http://127.0.0.1:8770/inference`

Multipart form. `file` is a WAV; 16-bit PCM mono is what the app records.

| Field | Value |
|---|---|
| `file` | WAV audio |
| `language` | `ja` |
| `response_format` | `verbose_json` for confidence, `json` for text only |

```bash
curl -sS -X POST http://127.0.0.1:8770/inference \
  -F "file=@water.wav" -F language=ja -F response_format=verbose_json
```

Response (trimmed to the parts the app uses):

```json
{
  "text": "水をください\n",
  "detected_language": "japanese",
  "detected_language_probability": 0.9918,
  "segments": [
    {
      "text": "水をください",
      "avg_logprob": -0.0963,
      "no_speech_prob": 0.0000028,
      "temperature": 0.0,
      "words": [
        {"word": "水", "probability": 0.8619, "start": 0.05, "end": 0.47}
      ]
    }
  ]
}
```

Confidence is available, but **do not gate no-speech on it**: pure digital silence
returns `(音楽)` with `avg_logprob ≈ 0` and `no_speech_prob ≈ 0`. The energy gate on
the captured waveform is the authoritative no-speech check; `avg_logprob` is a useful
secondary signal for "transcript looks shaky", and `no_speech_prob` is only a hint.

### TTS — `POST http://127.0.0.1:8771/v1/audio/speech`

JSON body, returns WAV bytes.

```bash
curl -sS -X POST http://127.0.0.1:8771/v1/audio/speech \
  -H 'Content-Type: application/json' \
  -d '{"input":"水をください。","voice":"jf_alpha"}' -o out.wav
```

| Field | Default | Notes |
|---|---|---|
| `input` | — | Japanese text |
| `voice` | `jf_alpha` | `jf_*`/`jm_*` Japanese, `af_*`/`am_*` English |
| `response_format` | `wav` | |
| `speed` | `1.0` | |

Response headers: `X-Generation-Seconds`, `X-Audio-Seconds` (useful for cache decisions
and for pre-generating upcoming lesson audio).

`GET /health` → `{"status":"ok","model":"kokoro-v1.0.onnx","intra_op_threads":4}`

Japanese voices available: `jf_alpha`, `jf_gongitsune`, `jf_nezumi`, `jf_tebukuro`,
`jm_kumo`.

The service is non-streaming: the whole WAV is produced before the response starts.

## Verifying it is not publicly exposed

```bash
ss -ltn | grep -E '8770|8771'
# expect two lines, both 127.0.0.1 — never 0.0.0.0
```

Inside the containers the servers listen on `0.0.0.0`, which is why the host side of
each port mapping is pinned to `127.0.0.1`. Nothing outside the machine can reach them.

## Benchmarks

See [BENCHMARKS.md](BENCHMARKS.md) for measured numbers and the deviation notes
(kokoro-onnx is espeak-only for G2P, so this service supplies misaki's Japanese G2P).

```bash
./bench-stt.sh /path/to/*.wav    # per-file latency + confidence
./bench-stt-matrix.sh /tmp/bench # Small vs Medium in one CSV
./bench-tts.sh                   # FP32/INT8 x threads 2/4/6
./bench-roundtrip.sh             # TTS -> STT intelligibility check
```
