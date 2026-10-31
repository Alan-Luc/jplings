#!/usr/bin/env bash
# Measures whisper.cpp server latency and confidence for each WAV given, three
# timed runs each after one warm-up.
# Prints CSV: file,median_ms,avg_logprob,detected_lang_prob,text
set -euo pipefail

url=${JPLINGS_STT_URL:-http://127.0.0.1:8770/inference}

now_ms() { date +%s%N | cut -b1-13; }

for f in "$@"; do
  # Warm-up: the first request after load pays one-off allocation costs.
  curl -fsS -o /dev/null -X POST "$url" \
    -F "file=@$f" -F response_format=json -F language=ja

  times=()
  body=''
  for _ in 1 2 3; do
    start=$(now_ms)
    body=$(curl -fsS -X POST "$url" \
      -F "file=@$f" -F response_format=verbose_json -F language=ja)
    end=$(now_ms)
    times+=($((end - start)))
  done

  median=$(printf '%s\n' "${times[@]}" | sort -n | sed -n 2p)
  text=$(grep -o '"text":"[^"]*","segments"' <<<"$body" | head -1 |
    sed -e 's/^"text":"//' -e 's/","segments"$//')
  logprob=$(grep -o '"avg_logprob":[-0-9.e]*' <<<"$body" | head -1 | cut -d: -f2)
  langprob=$(grep -o '"detected_language_probability":[-0-9.e]*' <<<"$body" | cut -d: -f2)
  printf '%s,%s,%s,%s,%s\n' "$(basename "$f")" "$median" "$logprob" "$langprob" "$text"
done
