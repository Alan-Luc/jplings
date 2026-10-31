#!/usr/bin/env bash
# Round-trip intelligibility check: synthesize each sentence with Kokoro, transcribe
# it with whisper.cpp, and compare against the source text. This is a lower bound on
# intelligibility — it measures whether the speech is recoverable, not whether it
# sounds natural to a Japanese speaker.
#
# Prints CSV: model,threads,sentence_id,input,transcript,match
set -euo pipefail

cd "$(dirname "$0")"

SENTENCES=(
  "水をください。"
  "すみません、これはいくらですか。"
  "私は毎日日本語を勉強しています。"
  "昨日は友達とラーメンを食べました。"
  "駅はどこにありますか。"
)

normalize() { tr -d '[:space:]。、？！?！' <<<"$1"; }

wait_healthy() {
  for _ in $(seq 1 90); do
    curl -fsS http://127.0.0.1:8771/health >/dev/null 2>&1 && return 0
    sleep 1
  done
  echo "kokoro did not become healthy" >&2
  return 1
}

echo "model,threads,sentence_id,input,transcript,match"
for model in kokoro-v1.0.onnx kokoro-v1.0.int8.onnx; do
  threads=4
  KOKORO_MODEL=$model ORT_INTRA_OP_THREADS=$threads \
    sg docker -c "cd $(pwd) && KOKORO_MODEL=$model ORT_INTRA_OP_THREADS=$threads docker compose up -d --force-recreate kokoro" >/dev/null 2>&1
  wait_healthy

  for i in "${!SENTENCES[@]}"; do
    wav=$(mktemp --suffix=.wav)
    curl -fsS -o "$wav" -X POST http://127.0.0.1:8771/v1/audio/speech \
      -H 'Content-Type: application/json' \
      -d "$(printf '{"input":"%s","voice":"jf_alpha"}' "${SENTENCES[$i]}")"

    transcript=$(curl -fsS -X POST http://127.0.0.1:8770/inference \
      -F "file=@$wav" -F response_format=json -F language=ja |
      sed -e 's/^{"text":"//' -e 's/"}$//' -e 's/\\n//g')

    if [[ "$(normalize "${SENTENCES[$i]}")" == "$(normalize "$transcript")" ]]; then
      match=yes
    else
      match=no
    fi
    echo "$model,$threads,$((i + 1)),${SENTENCES[$i]},$transcript,$match"
    rm -f "$wav"
  done
  echo "# ${model} container memory: $(sg docker -c 'docker stats --no-stream --format "{{.MemUsage}}" jplings-kokoro')" >&2
done
