# Placement diagnostic

`content/placement.yaml` is 16 items: 8 recognition (audio-first, answered by meaning)
and 8 production (typed). It loads before `unit-*.yaml`, so a session started from the
top runs it first. Budget 10 minutes.

Run it with `./dev run`. The items are diagnostic only: their concepts carry a
`placement-` prefix, so nothing here satisfies or blocks a teaching concept's mastery or
skip gate, and no item is a lesson.

## Reading the result

Score recognition out of 8 and production out of 8, and read them separately.
Recognition outruns production in every learner, and only production justifies starting
later — never recognition alone.

| Recognition | Production | Start point |
|---|---|---|
| 8 | 7–8 | U3 L1 (week 2). Test out U1–U2: one diagnostic, one or two production items, on. |
| 7–8 | 4–6 | U1 L3 (week 1), with U1 L1–L2 condensed to a production check each. |
| 5–6 | 2–3 | U1 L1, standard pace. Watch U3 (verb groups) closely — it is the usual blocker. |
| ≤4 | ≤1 | U1 L1, and hold 3 lessons a week until delayed checks recover (PLAN.md §3). |

Every wrong item that names a systematic error — a particle choice, a verb class, a
missing っ — is written into the error log. Those are claims about his misconceptions,
not about today's pace, and PLAN.md §3 reads them when deciding to drop to 3 lessons.

## What it deliberately does not measure

- **Pronunciation quality.** Never scored, here or anywhere. A spoken answer is graded on
  word identity, so a correct word in a poor accent is correct.
- **Kanji reading.** Items display kana and kanji with romaji; reading kanji is not a
  trip-critical skill in this plan.
- **Speaking under time pressure.** The production items are typed, because the
  diagnostic measures whether a form exists, and typing it proves the form rather than
  fluency. Fluency and repair get measured by the units' `conversation` scenarios.
- **Listening at conversational speed.** The items are single sentences, which is the
  comprehension floor, not the ceiling.
