#!/usr/bin/env bash
# Kokoro TTS benchmark matrix: FP32 vs INT8 across ONNX intra-op thread counts.
# Prints CSV: model,threads,sentence_id,generation_s,audio_s,rtf
#
# Restarts the compose service for each configuration so thread and model settings
# actually take effect.
set -euo pipefail

cd "$(dirname "$0")"

SENTENCES=(
  "水をください。"
  "すみません、これはいくらですか。"
  "私は毎日日本語を勉強しています。"
  "昨日は友達とラーメンを食べました。"
  "駅はどこにありますか。"
)

wait_healthy() {
  for _ in $(seq 1 90); do
    if curl -fsS http://127.0.0.1:8771/health >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "kokoro did not become healthy" >&2
  return 1
}

echo "model,threads,sentence_id,generation_s,audio_s,rtf"
for model in kokoro-v1.0.onnx kokoro-v1.0.int8.onnx; do
  for threads in 2 4 6; do
    KOKORO_MODEL=$model ORT_INTRA_OP_THREADS=$threads \
      sg docker -c "cd $(pwd) && KOKORO_MODEL=$model ORT_INTRA_OP_THREADS=$threads docker compose up -d --force-recreate kokoro" >/dev/null 2>&1
    wait_healthy

    # One untimed request so the session is warm before measuring.
    curl -fsS -o /dev/null -X POST http://127.0.0.1:8771/v1/audio/speech \
      -H 'Content-Type: application/json' \
      -d '{"input":"テスト。","voice":"jf_alpha"}'

    for i in "${!SENTENCES[@]}"; do
      headers=$(mktemp)
      curl -fsS -o /dev/null -D "$headers" -X POST http://127.0.0.1:8771/v1/audio/speech \
        -H 'Content-Type: application/json' \
        -d "$(printf '{"input":"%s","voice":"jf_alpha"}' "${SENTENCES[$i]}")"
      generation=$(grep -i '^x-generation-seconds:' "$headers" | tr -d '\r' | awk '{print $2}')
      audio=$(grep -i '^x-audio-seconds:' "$headers" | tr -d '\r' | awk '{print $2}')
      rtf=$(awk -v g="$generation" -v a="$audio" 'BEGIN { if (a > 0) printf "%.3f", g / a; else print "nan" }')
      echo "$model,$threads,$((i + 1)),$generation,$audio,$rtf"
      rm -f "$headers"
    done
  done
done
