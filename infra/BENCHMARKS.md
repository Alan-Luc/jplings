# S0.1 speech stack benchmarks

Measured on the target host, 2026-10-31. Every number here came from a real run of
the containers in this directory; nothing is quoted from external benchmarks except
where explicitly labelled as such.

## Environment

| Component | Value |
|---|---|
| OS | NixOS 25.11 |
| CPU | AMD Ryzen 5 5600, 6 cores / 12 threads |
| RAM | 15.5 GiB usable |
| GPU | AMD Radeon RX 9060 XT, 15.9 GiB VRAM, `RADV GFX1200` |
| Mesa / Vulkan | Mesa 25.2.6, Vulkan instance 1.4.328 |
| Docker / Compose | 29.5.3 / 2.40.3 |

Vulkan stack confirmed on the host with `vulkaninfo --summary`:

```
deviceName = AMD Radeon RX 9060 XT (RADV GFX1200)
driverVersion = 25.2.6
deviceID = 0x7590
```

## Verdict on the Phase 0 questions

| Question | Result |
|---|---|
| Vulkan GPU acceleration on RDNA4 (RK1) | **Works.** whisper.cpp logs `ggml_vulkan: 0 = AMD Radeon Graphics (RADV GFX1200)` and `using Vulkan0 backend`. No CPU fallback needed, so criterion 3 stands unrelaxed. |
| Kokoro Japanese on CPU (RK2) | **Works**, but only after fixing G2P — see "Deviations" below. |
| Per-utterance confidence exposed | **Yes.** `response_format=verbose_json` returns per-segment `avg_logprob` and `no_speech_prob`, per-word `probability`, and `detected_language_probability`. The low-confidence branch in S1.4 can be live. |
| Both services concurrent within 16 GB (criterion 7) | **Yes**, with wide margin: 6.8 GiB system RAM used, 2.9 GiB VRAM, 9.1 GiB still available. |

## Whisper.cpp (STT, Vulkan)

Image: `ghcr.io/ggml-org/whisper.cpp:main-vulkan` (libwhisper 1.9.5, ggml 0.28.0).
Weights are bind-mounted from `./models`.

Audio set: five Japanese sentences synthesized with Kokoro (`jf_alpha`), held in
`/tmp/bench/s1..5.wav`. Three timed runs per file after one warm-up; the table
reports the median.

### Small vs Medium

| Model | Vulkan weight size | Latency (ms, median of 3) | Word-level accuracy |
|---|---|---|---|
| `ggml-small.bin` | 487 MiB | 398, 401, 441, 440, 405 — **mean 417 ms** | 5/5 |
| `ggml-medium.bin` | 1533 MiB | 1157, 1246, 1288, 1284, 1218 — **mean 1239 ms** | 5/5 |

Accuracy differences were punctuation only: Small drops `、` and sometimes `。`,
Medium adds `?` and `。`. Both preserved every word and particle.

| Model | `avg_logprob` range | `detected_language_probability` |
|---|---|---|
| Small | −0.025 … −0.177 | 0.987 – 0.994 |
| Medium | −0.016 … −0.281 | 0.982 – 0.997 |

**Recommendation: Small.** It is ~3x faster with no observed accuracy loss, which
matters more than Medium's marginal confidence. This is provisional in the sense the
plan already anticipates: the audio was synthetic and clean, so this sample rejects a
clearly bad profile but does not certify accuracy on learner speech.

Latency varied between measurement sessions (an earlier isolated run of Small gave
226–289 ms with Kokoro idle and `response_format=json`). The table above was
collected with both containers loaded and `verbose_json`, so treat ~420 ms as the
realistic interactive figure and ~230 ms as the floor.

### The silence finding (validates a plan assumption)

Three seconds of digital silence, submitted with `language=ja`:

```json
{"text":"(音楽)\n","segments":[{"avg_logprob":-0.001038,"no_speech_prob":0.000012475,...}]}
```

Whisper reported "（music）" with **near-perfect confidence** and
`no_speech_prob ≈ 0`. This is exactly the failure mode the plan cites when it
requires an independent energy/silence gate instead of trusting Whisper confidence.
The empirical evidence confirms the gate is mandatory, not defensive.

Note the output was a bracketed non-speech annotation, so a cheap secondary filter on
`(…)`/`[…]` tokens is also available — but it is not a substitute for the energy gate.

## Kokoro ONNX (TTS, CPU)

Built locally from `./kokoro-onnx` (see "Deviations"). Includes the Japanese G2P that
the upstream ONNX package lacks.

| Variant | File | Size |
|---|---|---|
| FP32 | `kokoro-v1.0.onnx` | 311 MB |
| INT8 | `kokoro-v1.0.int8.onnx` | 89 MB |
| Voices | `voices-v1.0.bin` | 27 MB |

### FP32 vs INT8 across thread counts

Real-time factor (generation seconds / audio seconds), mean of five sentences.
Lower is better; below 1.0 is faster than real time.

| Model | 2 threads | 4 threads | 6 threads |
|---|---|---|---|
| FP32 | 0.445 | 0.276 | 0.229 |
| INT8 | 0.352 | 0.231 | 0.196 |

Absolute generation time for the shortest (1.2 s) and longest (3.0 s) sentences at
4 threads: FP32 0.376 s / 0.757 s, INT8 0.304 s / 0.650 s.

| Model | Container memory |
|---|---|
| FP32 | 526 MiB |
| INT8 | 284 MiB |

INT8 is ~20 % faster and uses about half the memory. Going from 2 to 4 threads buys
far more than 4 to 6 (FP32: 0.445 → 0.276 → 0.229), so 4 threads is the sensible
default and leaves CPU for the rest of the desktop.

Full raw matrix: rerun `./bench-tts.sh`.

### Time to first audio

This service is **non-streaming**, so time-to-first-audio equals total generation
time. Measured with curl `time_starttransfer` vs `time_total`:

| Sentence | TTFB | Total |
|---|---|---|
| 駅はどこにありますか。(1.6 s audio) | 0.412 s | 0.412 s |
| 私は毎日日本語を勉強しています。(3.0 s audio) | 0.660 s | 0.660 s |

TTFB and total are identical to within 0.3 ms because the whole WAV is synthesized
before any byte is sent. Streaming would be required to get first audio earlier; per
the plan that complexity is deferred until measurements justify it.

Both a female (`jf_alpha`) and male (`jm_kumo`) Japanese voice synthesize correctly.

### Intelligibility

I cannot judge Japanese naturalness — neither I nor this machine is a native or
fluent listener, so any "sounds natural" claim would be fabricated. What was actually
measured is whether the speech is *recoverable*: synthesize with Kokoro, transcribe
with Whisper, compare to the source text.

| Variant | Round-trip matches |
|---|---|
| FP32 @ 4 threads | 5/5 |
| INT8 @ 4 threads | 5/5 |

Both variants recovered every word and particle of all five sentences. That
establishes the audio is intelligible to a strong ASR model; it does **not**
establish naturalness or a pleasant accent. Default is FP32 on fidelity grounds
since INT8's speed advantage is not needed at these latencies.

Raw results: rerun `./bench-roundtrip.sh`.

## Concurrent resource use

| Scenario | System RAM used | RAM available | VRAM used | Idle CPU |
|---|---|---|---|---|
| Baseline, no containers | 6099 MiB | 9818 MiB | 1422 MiB | — |
| Small + Kokoro INT8 | 6571 MiB | 9346 MiB | 1934 MiB | whisper 0.00 %, kokoro 0.10 % |
| Medium + Kokoro INT8 | 6783 MiB | 9134 MiB | 2921 MiB | whisper 0.00 %, kokoro 0.10 % |

Container RSS is small (whisper 35–46 MiB, kokoro 199–284 MiB) because Whisper's
weights live in VRAM, not system memory: the VRAM delta from baseline (512 MiB for
Small, 1499 MiB for Medium) matches whisper.cpp's own reported weight sizes.

Idle CPU is effectively zero for both services, matching the handoff expectation that
Kokoro is quiet between requests.

## Deviations from the handoff's stated implementation

**kokoro-onnx is English-only for G2P.** The upstream `kokoro-onnx` package
phonemizes with espeak, so Japanese input produces nonsense — 水をください came out
of Whisper as おくうしえ. The service in `./kokoro-onnx` therefore uses misaki's
Japanese G2P (`JAG2P`) and calls Kokoro with `is_phonemes=True`. After that fix the
same sentence round-trips exactly.

**misaki's Japanese G2P needs a unidic dictionary, and the right one matters.**
`unidic-lite` is a reduced dictionary that mis-segments geminated digraphs: はっぴゃく
came out as `haʔpʲi ja kɯeɴ` ("ha-pyi-ja-kuen") and Whisper heard ハッピー役演. The full
`unidic` package ships without a dictionary and fails at import, so the Dockerfile
installs `misaki[ja]` and runs `python -m unidic download` at build time. With the full
dictionary the phonemes are `haʔpʲakɯ eɴ desɨ` and the sentence round-trips as 800円,
matching the PyTorch implementation exactly.

### Content authoring rule: write numbers in kanji

Kana-spelled counter readings still mis-tokenize upstream — ろっぴゃく alone yields
`ɾoʔpʲi` and is heard as のんぴゃく / 6ピクエン. **The PyTorch implementation has the
identical bug**, so it is an upstream misaki/unidic limitation, not an ONNX one.
Authoring numbers as kanji numerals routes the reading through the dictionary instead
of the kana tokenizer:

| Input | Phonemes | Round-trip |
|---|---|---|
| `六百円です。` | `ɾoʔpʲakɯ eɴ desɨ` | 600円です |
| `八百円です。` | `haʔpʲakɯ eɴ desɨ` | 800円です |
| `三千五百円です。` | `saɴʣeɴ ɡoçakɯ eɴ desɨ` | 3500円です |
| `ろっぴゃく` | `ɾoʔpʲi` | のんぴゃく (wrong) |

Lesson content must therefore author TTS number prompts in kanji. This is a content
rule, not a code workaround, and it costs nothing since authored readings already
carry the kana form for display.

**The prefabricated PyTorch Kokoro image was rejected.** `ghcr.io/remsky/kokoro-fastapi-cpu`
runs and its Japanese output round-trips correctly, but it is PyTorch rather than ONNX
Runtime, exposes neither FP32/INT8 selection nor ONNX thread control, and so cannot
answer the benchmarks §8A actually asks for. Measured for reference at 4 threads: it
generated 1.47 s of audio in 0.545 s (RTF 0.37) and used 1.48 GiB of RAM — heavier and
slower than the ONNX build, consistent with the external comparison in §8A.

**Thread counts are ONNX intra-op threads.** kokoro-onnx constructs its own
`InferenceSession` with no thread options, so `server.py` patches the session factory
to apply `ORT_INTRA_OP_THREADS`. This makes the 2/4/6 comparison a genuine measurement
of ONNX intra-op threading rather than a CPU-quota approximation.

## Not verified here

- **STT accuracy on real learner speech.** The plan asks for ~20 learner utterances
  captured with `pw-record`. Only Alan can produce those, so accuracy on beginner
  pronunciation, hesitation, and grammatical errors is **Not run**. All STT accuracy
  numbers above use synthetic speech and should be treated as an upper bound.
- **Japanese naturalness / accent quality.** Not assessable without a fluent listener.
- **S0.2 learner baseline diagnostics.** Separate slice, not started.
