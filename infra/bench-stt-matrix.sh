#!/usr/bin/env bash
# Runs the STT benchmark for each whisper model size and prints one combined CSV.
# Usage: bench-stt-matrix.sh <audio-dir>
set -euo pipefail

cd "$(dirname "$0")"
audio_dir=${1:?usage: bench-stt-matrix.sh <audio-dir>}
shopt -s nullglob
files=("$audio_dir"/*.wav)
if [ ${#files[@]} -eq 0 ]; then
  echo "no wav files in $audio_dir" >&2
  exit 1
fi

echo "model,file,median_ms,avg_logprob,lang_prob,text"
for model in ggml-small.bin ggml-medium.bin; do
  sg docker -c "cd $(pwd) && WHISPER_MODEL=$model docker compose up -d --force-recreate whisper" >/dev/null 2>&1

  # Wait for the server: the retry doubles as the model load wait.
  for _ in $(seq 1 120); do
    curl -fsS -o /dev/null -X POST http://127.0.0.1:8770/inference \
      -F "file=@{files[0]}" -F language=ja 2>/dev/null && break
    sleep 1
  done

  while IFS= read -r line; do
    echo "$model,$line"
  done <<<"$(bash bench-stt.sh "${files[@]}")"
done
