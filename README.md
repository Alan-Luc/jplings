# jplings

A personalized Japanese learning system: a Rustlings-style loop (learn → exercise →
feedback → demonstrate → progress → revisit) for one learner, with local speech.

## What is here

| Path | Purpose |
|---|---|
| `content/` | Declarative curriculum (units → lessons → steps) in YAML |
| `internal/content` | Schema and lint for that content |
| `internal/romaji` | Answer matching: spelling conventions, and near-miss detection |
| `internal/grade` | Deterministic graders for every exercise kind |
| `internal/convo` | Scripted conversation interpreter |
| `internal/profile` | SQLite learner record: evidence, mastery gates, pending queue |
| `internal/schedule` | SM-2 review plus the delayed retention queue |
| `internal/session` | Session engine: grade a step, record what it proved |
| `internal/speech` | Speech client interface, HTTP clients, energy gate, confirmation turn |
| `internal/audio` | Push-to-talk recording (`pw-record`) and playback (`mpv`) |
| `internal/eval` | DeepSeek rubric grader for open-ended production |
| `internal/tui` | Bubble Tea screens |
| `infra/` | Docker Compose speech stack — see [infra/README.md](infra/README.md) |

## Running

```bash
# 1. Bring up the speech stack (see infra/README.md for the model downloads)
cd infra && sg docker -c 'docker compose up -d' && cd ..

# 2. Check prerequisites
go run . doctor

# 3. Validate content
go run . lint content/

# 4. Start or resume a session
go run .                  # no API key needed; free-form steps then save ungraded
DEEPSEEK_API_KEY=... go run .   # enables grading of open-ended production

# 5. Grade anything that was saved ungraded
go run . grade --pending
```

The session cursor is saved after every step, so a session resumes exactly where it
stopped. Profile data lives in `$XDG_DATA_HOME/jplings/profile.db`
(default `~/.local/share/jplings/profile.db`), backed up on clean exit.

## Tests

```bash
go test ./...                 # hermetic: no containers needed
go test -tags live ./...      # against the real speech containers
```

## Design notes worth knowing before changing things

- **Only exact matches earn credit.** Near misses that differ by vowel length or
  gemination are reported by name and re-asked; they never mint evidence.
- **Silence is detected from the waveform, not the recogniser.** Measured on this
  machine, Whisper turns three seconds of digital silence into `(音楽)` at
  near-perfect confidence, so the energy gate is load-bearing.
- **Every spoken response is confirmed before grading**, and two denied transcripts
  offer a typed fallback rather than deadlocking progress.
- **Mastery needs production + generalization + retention**, and LLM-graded evidence
  counts only after the audit promotes that dimension to trusted.
- **Numbers authored for TTS should use kanji numerals.** Kana counter spellings
  mis-tokenize upstream; see [infra/BENCHMARKS.md](infra/BENCHMARKS.md).
