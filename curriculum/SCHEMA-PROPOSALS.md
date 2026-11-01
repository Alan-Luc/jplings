# Schema proposals

Status: proposals only. No engine change is made by this document, and no content file
may use a field listed here until it exists in `internal/content/schema.go`.

There is no `docs/CONTENT-SCHEMA.md` in the repository, so the contract I designed
against is the code at `3a35f53` (`internal/content/schema.go`, `load.go`, `coverage.go`,
`internal/grade/grade.go`, `internal/tui/tui.go`). Where the brief and the code disagree,
the code wins and the disagreement is recorded in §6.

Each proposal gives the smallest change that removes a real obstacle, an example of the
content it unlocks, and the reason it matters for learning. Everything the current schema
can already express is listed in §4 and needs no work at all.

---

## 1. Facts verified against the code

| Claim | Verdict | Where |
|---|---|---|
| YAML only, `.yaml`/`.yml`, strict decoding | true | `load.go: collect`, `KnownFields(true)` |
| Kinds are exactly the seven listed | true | `schema.go: kinds` |
| A step's concepts must be declared in its own unit | true | `schema.go: Step.validate` vs `u.Concepts` |
| Japanese text is `kanji`/`kana`/`romaji`; kana and romaji are required on every `Text` | true | `schema.go: Text.validate` |
| Written answers: exact match on `accept`, spacing ignored, punctuation ignored | true, and slightly more forgiving than the brief says | `grade.Written` → `romaji.Compare` |
| English answers: near-exact string match on `accept_meanings` | true | `grade.Meaning` → `normalizeMeaning` |
| `speaking` grades with `accept` **or** `rubric`, never both | true | `schema.go`, enforced at lint time |
| `off_target` is graded for written answers and for speech | true | `grade.offTarget`, `grade.offTargetSpeech` |
| Concepts are per-unit, so nothing is shared | true | `schema.go`, `coverage.go` |
| Listening is dictation | true, and it is forced | see P1 |
| Audio never plays for any kind except `listening` | true | `tui.go: audioText` returns "" unless `Kind == Listening` |
| `speaking` and `conversation` carry a spoken rubric with 4 dimensions scored 0–2 | true | `internal/eval`, `Rubric.Validate` |
| `Turn.Hints` exists in the schema and is read by nothing | true — dead field | `schema.go:40`, no consumer in `internal/` |
| Step-level audio provenance exists (`audio_source`, `audio_license`, `audio_attribution`) | true | `schema.go` |

Two probe results that drive P1:

- A `listening` step with `accept_meanings` and no `accept` fails lint with three errors
  (`needs a target`, `needs at least one accepted answer`, `needs an explanation`).
  Meaning-graded listening cannot be authored today.
- An `interpretation` step with an `audio:` field and `accept_meanings` passes lint
  cleanly and **never plays the audio**. This is the dangerous case: the file looks
  audio-first and silently becomes a reading exercise. I hit this while probing, not by
  reading.

---

## 2. Proposals, in priority order

### P1 — Comprehension answered by meaning, with the audio actually played

**Why it matters for learning.** The lesson template's whole comprehension track is
"hear it, answer with the meaning". Today the only way to hear something is `listening`,
which grades romanised dictation. Dictation measures the ability to transliterate what
was heard, exactly the skill the brief bans as a crutch and the one that does nothing for
a trip to Japan. It also fails the learner on を/お and ず/づ, which he does not need to
write. Meanwhile the one thing 19 weeks of listening practice must build — understanding
at speed — is the one thing the app cannot currently test without a written form.

This single change also gives the vocabulary requirement (§3, item 8: 10–15 words per
unit, audio-first, answered by meaning) for free. A vocabulary item is an audio-first
meaning question on one word.

**Smallest change.**

1. `grade.Meaning` is already the right grader; the session picks a grader from the step
   kind (`grade.Dimension` and the kind switch in `internal/session`). Add: if a step
   carries `accept_meanings`, grade with `Meaning` whatever its kind.
2. `tui.audioText`: play the target when the step's grading path is meanings, not only
   when `Kind == Listening`.
3. `schema.go`: allow `listening` (or an existing kind) to satisfy validation with
   `accept_meanings` in place of `accept`, and reject a step that has both grading paths.
4. Play the accepted answer's audio after a miss, on every production kind. Same
   synthesizer, no new field; the written answer is already shown, and the recording is
   the reference he compares his own speech against. Pronunciation is never scored, so
   this is a reference and not a penalty.

Example unlocked content — audio comprehension, no romaji, no dictation:

```yaml
- id: u4-l2-3
  kind: listening
  concepts: [n-de-place, exist-arimasu]
  prompt: What is being said about the place?
  audio: audio/unit-4/u4-l2-3.opus
  target:
    kanji: 駅の近くにコンビニがあります
    kana: えきのちかくにコンビニがあります
    romaji: eki no chikaku ni konbini ga arimasu
  accept_meanings:
    - There is a convenience store near the station.
    - Near the station there's a convenience store.
  explanation: >-
    に marks where the thing exists; が marks the thing that exists. A shop that exists
    somewhere takes に, not で — で marks where an action happens.
```

And an audio-first vocabulary card, same mechanism:

```yaml
- id: u4-l2-8
  kind: listening
  concepts: [word-mochikaeri]
  prompt: What does this word mean?
  audio: audio/unit-4/u4-l2-8.opus
  target: { kanji: 持ち帰り, kana: もちかえり, romaji: mochikaeri }
  accept_meanings: [takeaway, to take home, take-out]
  explanation: 持ち帰り is what a shop asks when you might eat the food elsewhere.
```

**Cost and risk.** Small: one validation branch, one grader selection, one `audioText`
condition. The risk is that `accept_meanings` stays near-exact, so P6's variants rule
applies — several natural English phrasings must be listed per item.

**Workaround today.** None that is honest. Authoring these as `interpretation` produces a
silent reading exercise, so I will not do it. Until P1 lands, comprehension is
`interpretation` (read, answer by meaning) and audio practice happens outside the app.

---

### P2 — Concepts shared across units

**Why it matters for learning.** Every one of the seven priority items in the brief is a
system that recurs in all ~16 units: に vs で is decided in the transport unit, the
restaurant unit and the hotel unit; て-form is needed by week 6 and still being drilled by
week 15; the repair toolkit comes back in every roleplay. With per-unit concepts, each
reappearance must invent a new concept ID (`particle-ni-u4`, `particle-ni-u9`, …). The
consequences are not cosmetic:

- `go run . coverage` reports each fragment separately, so a concept taught to a real
  standard in five units reads as five thin concepts or five shallow ones.
- Mastery gates and the spaced-review queue key on concept identity, so the same knowledge
  gets five independent, mostly-empty schedules. Review then repeats the same stimulus
  after 3 days for one fragment while the real skill decays.
- The learner's profile cannot answer "do I actually know に vs で", which is the single
  most useful question the profile could answer.

**Smallest change.** Validate a step's concepts against the union of concepts declared in
all loaded units instead of only its own unit, and have `coverage` aggregate a concept
across units into one row. Both are inside `internal/content`. No content file changes.

```yaml
# unit-4.yaml declares nothing new; it reuses particle-ni-vs-de from unit-3
- id: u4-l3-2
  kind: speaking
  concepts: [particle-ni-vs-de]
  prompt: Say "I study at the library" aloud.
  accept:
    - { kanji: 図書館で勉強します, kana: としょかんでべんきょうします, romaji: toshokan de benkyou shimasu }
```

**Cost and risk.** Small, but it moves the declaration point for concepts out of the file
that owns them, so the lint message for a typo becomes "undeclared anywhere" rather than
"not in this unit". That is a better message. A shared `content/concepts.yaml` is the
tidier version and I am not proposing it: it buys nothing over the union rule and adds a
file to keep in sync.

**Workaround today.** Duplicate concept IDs per unit, and accept that coverage and review
fragment. This is why the plan's unit count is a claim about lessons, not about concepts.

---

### P3 — A `teach` field on the lesson, rendered before its steps

**Why it matters for learning.** The learner's documented weakness is that he recognises
endings without being able to generate them, and that his particle errors are systematic
rather than random. Systematic errors are fixed by a rule he can state, not by more
exposure to items that assume the rule. `explanation` cannot do this job: it is feedback
attached to one step, shown after an answer, capped at a sentence or two of "why that was
wrong". A 150–300 word explanation with the misconceptions named has no home.

**Smallest change.** One field on `Lesson`, pointing at a sidecar file the loader already
ignores (`.md` is not collected), plus a TUI screen that shows it before the first step.

```yaml
- id: u3-l1
  title: The て-form as one system
  anchored: true
  teach: teach/u3-l1.md      # content/teach/u3-l1.md
  steps:
    - id: u3-l1-1
```

**Interim state, usable today.** The sidecar file `content/teach/u3-l1.md` is safe to write
right now — the loader only collects `.yaml`/`.yml`, so it cannot break lint. Writing the
files is not gated on this proposal. Reading them inside the app is.

---

### P4 — Voiced NPC lines in conversations

**Why it matters for learning.** A roleplay where the NPC's line is only printed is a
reading-and-replying drill. The stated goal is surviving spontaneous speech, and the
learner cannot hear what he never hears. The unvoiced turn is also the one place where
listening at conversational speed could be practised with a real conversational shape
(a question that expects an answer) rather than a single sentence.

**Smallest change.** No schema field. `Turn.Line` is already a `Text` with an optional
`kanji`. Synthesize it with the same client and voice the listening steps use, and require
`kanji` on NPC turns so counters are not spoken from kana (the brief's TTS hazard).

Risk: it needs the speech stack up for conversations, so conversations must degrade to
printed lines the way listening already degrades to the written prompt.

**Workaround today.** None. Conversations stay reading drills until this lands, so the plan
does not rely on them for listening practice; it treats them as production practice and
puts listening volume in `listening`/comprehension steps instead.

---

### P5 — An `anchored` flag on the lesson

**Why it matters for learning.** The brief carries a hypothesis to test, not to assume:
that explicit instruction connected to phrases he has heard for years in anime makes
concepts click faster for this learner. A hypothesis that is not marked in the data cannot
be tested after a month. One boolean on each lesson, plus delayed-check accuracy already
being recorded per concept, makes the comparison a query instead of an opinion.

**Smallest change.** `anchored: true|false` on `Lesson`, defaulting to false, surfaced in
the coverage/report output so the comparison is one filter. It changes no grading and no
session behaviour.

```yaml
- id: u3-l1
  title: The て-form as one system
  anchored: true           # its anchors are lines he has heard in anime
```

---

### P6 — More forgiving English meaning answers

**Why it matters for learning.** `normalizeMeaning` lowercases, trims, drops a trailing
period, and collapses punctuation. "It is 600 yen" and "600 yen" do not match; nor do
"There is a convenience store near the station" and "There's a convenience store near the
station". A learner who understands the sentence perfectly gets marked wrong, and the
feedback ("That is not the meaning here. Expected something like …") teaches him that his
comprehension is the problem. Repeated often enough, this is how a self-study tool loses
trust.

**Smallest change, in order of preference.**

1. Author the variants. Costs nothing, needs no code, and is what I will do regardless.
   Every meaning item lists the phrasings he is likely to type.
2. If that proves insufficient in use, allow `rubric` as a fallback on `interpretation` and
   `listening` **only after** the deterministic match fails, reusing `internal/eval`. A
   rubric score of 2 on `meaning` accepts; anything else shows the model's feedback plus
   the expected phrasings.

**Not proposed:** loosening the Japanese grader, or keyword-matching English. Keyword
matching accepts "There is no convenience store near the station" as correct, which is a
worse failure than a false negative.

---

## 3. Not proposed, and why

- **A `vocabulary` step kind.** P1 makes the existing kinds do the job: an audio-first card
  answered by meaning is a comprehension step whose "target" is one word. A new kind would
  need a new grader, a new TUI screen, a new coverage dimension, and a new `Dimension` entry
  in two files, and would still grade the same way.
- **A per-step `teach` field.** Teaching belongs to the lesson, not the step; per-step
  teaching would tempt 8 explanations into a lesson that needs one.
- **An `anchors` field.** Anchors are short lines he gives the meaning of, which is exactly
  an `interpretation` step with `accept_meanings`. They already work, they already count
  toward comprehension coverage, and they already fail honestly when he guesses wrong.
  Making them their own kind would only stop them counting as comprehension.
- **Any new gradable kind.** Each one costs an entry in `content.coverage.Dimension` and
  `grade.Dimension` — two places that must agree — plus a TUI branch.

---

## 4. What works today, unchanged

Designing around these keeps the first two units authorable with zero engine work.

- **Production by speaking.** `speaking` with `accept` for anything with a known correct
  form, `rubric` for truthful personal answers. This is the main production mode.
- **Variant tolerance for speech.** `normalizeJapan` strips punctuation and spacing and
  converts digits to kanji, and `off_target` names a valid-but-wrong-meaning answer. Listing
  every natural variant in `accept` is the supported way to be fair.
- **Recognition ≠ production, structurally.** Coverage already counts comprehension and
  production separately, and `Complete()` demands 1 comprehension and 2 production. That is
  the right shape for this learner's diagnosis and needs no change.
- **Anchors as comprehension steps.** With `accept_meanings`, as described above.
- **Tatoeba attribution** on imported sentences via `source:`, and on audio via
  `audio_source`/`audio_license`/`audio_attribution`.
- **A teaching sidecar file per lesson** at `content/teach/<lesson-id>.md`, ignored by the
  loader, readable today by a human and by P3 later.
- **Delayed checks** need no schema support: they are ordinary steps in a later lesson over
  unseen stimuli.

---

## 5. Where the measurement requirement can't be met today

The monthly benchmark (brief §5) has three parts. Two work; one doesn't.

- 10 unseen listening clips answered by meaning — blocked on **P1**.
- A 2-minute recorded monologue, transcribed and scored on clauses, verb forms and errors
  per clause — recording and transcription exist; the scoring is a human rubric and is
  written down in the plan, not coded.
- A 5-turn **unscripted** roleplay — the `conversation` kind is a fixed script matched by
  accept lists, so it cannot be unscripted. This needs an LLM partner (explicitly
  out of scope for this phase) or a human partner. The plan treats it as a human-partner
  benchmark and does not pretend the app can grade it.

---

## 6. Conflicts between the brief and the code

- The brief says grading is "close to exact-match" and that punctuation and spacing are not
  tolerated. The code is more forgiving than that: `normalizeJapan` and `romaji.Compare`
  ignore spacing, `。`, `、`, `！`, `？`, and accept digit forms of numerals. I designed
  against the code, so `accept` lists still include digit and punctuation variants.
- The brief expects "listening steps are dictation only" as a limit to design around. It is
  worse than a limit: dictation is the only possible reading of a `listening` step, and the
  obvious workaround (audio on an interpretation step) lints clean and plays nothing. See P1.
- The brief expects a missing teaching field and a missing vocabulary kind. Both are real,
  but only one extension is needed for both. See P1 and §3.
- `Turn.Hints` exists in the schema and nothing reads it. Either it is meant as a repair
  toolkit ("say もう一度") surfaced on a stall, or it should go. I am not relying on it.
