# jplings curriculum plan

Status: checkpoint 1. Plan only — no lesson content exists yet.
Written against `3a35f53` and the schema proposals in `SCHEMA-PROPOSALS.md`.
Learner: one adult, Cantonese L1, heavy prior anime/manga exposure, beginner
production. Trip: mid-March 2027.

## 1. The shape in one page

- **15 units, 60 lessons, weeks 1–15.** Plus **4 weeks of consolidation** (16–19),
  no new grammar.
- **Units are not the same size.** Conjugation gets 5 lessons, self-introduction gets
  4, hotel gets 3. Size is set by how much practice the task needs, never by a
  house style. A rigid 4-per-unit grid is the one thing I had in v1 and removed.
- **Three cumulative quizzes** carry their own lesson slots (Q1, Q2, Q3) and gate
  progress at 80%. See §6.
- **Eight practice shapes** are named, and every unit is required to use at least
  four of them. The dominant shape is error correction, not construction. See §5.
- Each lesson is ~25 minutes, so 60 lessons is ~25 hours. Available structured time
  is ~19 weeks × 5–9 h ≈ **130 hours**, meaning lessons are a fifth of the work and
  review, retry, listening and roleplay are the rest. **Nothing here is added to
  increase coverage.**
- Five things run as continuous tracks rather than owning units: question building,
  the repair toolkit, listening volume, katakana/kanji recognition, same-session
  retry.

Pace: 4 lessons a week, landing on the longer days. A 30-minute weekday fits review
only.

## 2. What is evidence and what is my judgement

**Established, from instructed-SLA literature:**

- Explicit instruction plus practice beats practice alone for adults (Norris &
  Ortega 2000; Spada & Tomita 2010).
- Corrective feedback that shows the correct form beats bare "wrong", and prompts to
  try again beat immediate correction (Li 2010; Lyster & Saito 2010).
- Spaced retrieval retains better than massed practice (Cepeda et al. 2006; Kang
  2016), and retrieval is itself a learning event (Roediger & Karpicke 2006).
- High-frequency vocabulary and formulaic chunks carry disproportionate early
  communicative return (Nation).
- Recognition outruns production consistently.

**Borrowed from rustlings' curriculum structure**, which is a programming drill course
and therefore homology, not evidence:

- Uneven section sizes set by difficulty.
- Error-fixing as the dominant exercise shape.
- Cumulative quizzes as gates.
- A concept explanation (section README) before the exercises that use it.
- Opening each section with an item that is one token from correct.

I am taking the structure and not the execution: no manifest file, no per-exercise
hint, no file-editing loop, no done/not-done flags. Those exist to serve a compiler.

**My design judgement, not established findings:**

- The exact counts: 60 lessons, 25 minutes, 4 per week, 10–15 words per unit.
- The unit order, and which travel task owns which grammar.
- That conjugation should arrive as one system in weeks 2–6 rather than spread.
- That anime-anchored lessons will click faster for him. This stays a hypothesis;
  §9 describes the test.
- Plain-form-first sequencing over textbook ます-first. An inference from his input
  profile, not a finding.
- That 4 weeks of consolidation is the right allocation rather than 3.

## 3. Pace and adaptation

Fixed: 4 lessons per week, quiz lessons included.

**Drop to 3 lessons that week** when either holds, checked weekly:

- delayed-check accuracy on the previous week's lessons is below 80%, or
- daily due review takes more than 20 minutes.

**Return to 4** when both recover. **Never below 2** — the trip date does not move,
and at 2/week the correct response is to cut from §4, not to drift.

**Speed up** to 5–6 only when placement or a test-out shows he can already *produce*
the material. Recognition never justifies speed-up.

**A failed quiz** (§6) costs the next week's fourth lesson slot and converts it to
targeted review of the quiz's failed concepts. It never rewinds the course.

Test-out lessons are short: a diagnostic, one or two production items, then on. Weeks
1–3 will mostly be test-outs.

Weekly shape, whatever the lesson count:

| Slot | Time | Content |
|---|---|---|
| Daily review | 10–20 min | Due items, plus the error log grouped by misconception |
| New lessons | 25 min each | As scheduled |
| Weekend roleplay | 20–30 min | Two `conversation` scenarios from recent units |
| Listening volume | 10–15 min | Unseen clips; in-app path blocked, see §10 |

## 4. What gets compressed

Compressed means present, tested out, and given a short review lesson rather than
taught as new. Not skipped.

| Existing material | Treatment | Why |
|---|---|---|
| です / じゃないです noun sentences | Half of U1 L1 | He already builds these; the lesson's job is dropping 私は and は/が |
| は / を basics | U1 and U3 | を is only meaningful with a transitive verb, so it belongs with verbs |
| Numbers, hundreds, thousands, 円, 回 | **One lesson inside U8**, not a unit | Audit §3.1: three of unit B's four lessons teach his strongest area. Keep the sound changes (ろっぴゃく, はっせん, さんびゃく, いっかい) — those are real and he may have them wrong — and drop the rest to one listening check |
| Hiragana | Assumed; one item in Q1 | Solid per the assessment |
| Katakana | Continuous track | Needed for signs and menus; cheap in parallel |
| Counters beyond 一つ/二つ/〜枚/〜個/〜回 | Consolidation | Low return per hour for a two-week trip; a misheard counter is recoverable |

Cut entirely: kanji production, handwriting, typed composition, JLPT ordering,
textbook reading passages.

## 5. Practice shapes

The plan's central structural claim. Each shape drills a different thing, and a unit
that uses only one shape produces a learner who can do that one thing.

| Shape | What the learner does | Why it earns its place | Kind |
|---|---|---|---|
| **S1 Minimal contrast** | Two sentences differing by one particle or ending; say which means what | The assessment's single best-documented gap is を/に and は/が, which only exist in contrast. This is where は vs が is taught honestly | interpretation, listening |
| **S2 Substitution** | Same pattern, vocabulary already shown in the lesson | Builds the form without also carrying a vocabulary search | speaking |
| **S3 Transfer** | Same pattern, vocabulary **not** shown | The only evidence that the pattern is a rule and not a phrase | speaking |
| **S4 Error correction** | Hear or read a wrong sentence; say what is wrong, then say it right | Borrowed from rustlings' dominant shape. His errors are systematic, and naming an error is how a rule becomes explicit. Also the cheapest way to drill particles without blanks | interpretation → speaking |
| **S5 Interrogation** | Build the question a statement answers; answer a question he has not seen | Question formation is a required track from week 1 and otherwise never practised | speaking |
| **S6 Self-expression** | A truthful answer about himself | Speaking that survives contact with reality; graded by `rubric`, never `accept` | speaking |
| **S7 Scenario** | A `conversation` requiring the pattern, plus one unexpected follow-up | The trip skill, and the only place repair gets exercised | conversation |
| **S8 Delayed check** | Same concept, new stimuli, 2–3 days later | Retention evidence; without it mastery is a fiction, per the audit | any |

Rules that follow from the table:

- **Every unit uses S1, S3, S4 and S7 at minimum.** S1 for the concept it teaches, S3
  for production, S4 for its own misconception, S7 for use.
- **S4 comes before S3 within a unit.** Naming what is wrong is cheaper than
  generating, and it primes the contrast.
- **A unit's first lesson opens with S1**, never with construction: see the pattern
  working before being asked to build it.
- **Difficulty ramps inside the unit**: L1 contrast, L2–3 guided, L4 transfer, last
  lesson scenario. Across the course the ramp is the same but the scaffolding
  disappears — by U15 nothing is shown on screen before responding.
- **Every production item ends with the sound of the right answer.** After grading,
  the accepted answer's audio plays alongside the written form, and his own recording
  is never scored for pronunciation. He needs to hear the rendition he is being
  compared against, and pointing the app's own voice at the accepted answer is the
  speaking equivalent of rustlings' `solutions/` tree. It is not a pronunciation
  score; it is the reference. Needs the audio path from P1.

## 6. Quizzes

Three cumulative quizzes, one lesson slot each. They cross sections, share no
stimulus with any lesson, and have no scaffold.

| Quiz | After | Covers | Items | Gate |
|---|---|---|---|---|
| Q1 | U6 (week 7) | U1–U6: conjugation generation on unseen verbs, に vs で, question building, repair | 15: 8 production (6 spoken), 7 comprehension | 80% |
| Q2 | U11 (week 12) | U1–U11, weighted to the travel tasks | 15, same split | 80% |
| Q3 | U15 (week 15) | Whole course, in the monthly-benchmark format | Benchmark structure | 80% |

Q3 is also the February benchmark: one run, reported twice, because a separate
full-course quiz five days after a full-course benchmark measures fatigue.

**Items are interleaved, not blocked.** Consecutive items come from different units —
Q1 alternates U3 conjugation, U2 particles, U6 questions, U4 endings — because choosing
the right form against competitors is the skill, and a block of たい items followed by a
block of て items only practises retrieval in a queue it will never appear in.

A quiz failure is not a retake loop. It is a signal that the week's fourth slot goes
to the failed concepts, and Q1's concepts stay in the review queue at their normal
spacing.

## 7. The sequence

### Units

| Unit | Lessons | Task it performs | Grammar introduced | Vocabulary | Requires | Why here |
|---|---|---|---|---|---|---|
| U1 はじめまして | 4 | Introduce yourself and ask the same back | です/じゃないです, は vs が (identity), か, も, dropping 私は | 学生, 会社員, 先生, 名前, nationalities | — | Fastest thing to get right, and it fixes both errors the audit found in unit-a; every later roleplay opens with it |
| U2 何が好きですか | 3 | Say preferences and ask for someone's | 好き/きらい/ほしい + が, は vs が with すき, とても/あまり/ぜんぜん, の | 好き, きらい, ほしい, スポーツ, 音楽, 映画, コーヒー | U1 | が in its easiest, highest-frequency use before it has to carry subjects |
| U3 毎日すること | 5 | Narrate your day and what you did | **Verb groups + ます stem as one system**, ません/ました/ませんでした, に (time) vs で (place), を | 起きる, 寝る, 食べる, 飲む, 行く, 来る, する, 働く, 見る, 〜時, 〜分 | U1, U2 | The gap that blocks production everywhere; five lessons because the stem machine is what every later unit builds on |
| U4 〜たい・〜ない | 4 | Say what you want and don't want | たい, ない, ましょう/ましょうか, を vs が with たい/ほしい | 飲む, 食べる, 買う, 泳ぐ, 休む | U3 | Deliberate reversal of textbook order: he knows what たい *means* and cannot generate it, so it is built on the stem machine |
| U5 今していること | 5 | Say what you're doing and where you're going | **て-form as one system**, 〜ている, 〜てください, を vs に (乗る/会う/歩く) | 住む, 待つ, 持つ, 歩く, 乗る, 降りる, 会う, 教える | U3, U4 | The third stem completes the system; を vs に needs movement verbs to be meaningful, which this unit supplies |
| U6 聞き返す | 4 | Ask, clarify, recover | Question words, 〜んですか, 〜って何ですか, 〜は日本語で何と言いますか | 何, だれ, どこ, いつ, どう, どうして, ゆっくり, もう一度 | U1–U5 | The repair toolkit pays out from the moment he lands, and it needs the stems so he can ask about forms he can already hear |
| Q1 | 1 | Cumulative gate | — | — | U1–U6 | First point where enough grammar exists to compose an unseen sentence |
| U7 行き方 | 4 | Get from A to B, ask how long it takes | に/で/から/まで in contrast, 乗る/降りる/乗り換える, どのくらい/かかります | 駅, 電車, バス, 切符, 何番線, 乗り換える, かかる | U5 | Day one at the airport, and it drills the に vs で contrast the assessment singled out |
| U8 買い物 | 4 | Buy something and ask the price | これ/それ/あれ/どれ, この/その/あの, いくら, 一つ/二つ/〜枚/〜個, をください, はありますか, **the numbers lesson** | これ/それ/あれ, いくら, 円, 高い, 安い, 袋, レシート | U2, U3 | Prices recycle the compressed numbers material in a real task |
| U9 レストランで | 4 | Order, ask what's in a dish, handle a follow-up | お〜をください, 〜をお願いします, 〜はありますか, service keigo **recognition only** | 定食, 大盛り, 持ち帰り, おすすめ, アレルギー, お会計, お水 | U6, U8 | Needs U6's repair to survive アレルギーはございますか; the keigo is never produced |
| U10 ホテルで | 3 | Check in, leave luggage, ask permission | あります/います with に/が, 〜てもいいですか, 〜ないでください, 〜ています for states | 予約, 荷物, 預ける, チェックイン, 部屋, 空いている | U7, U9 | Existence verbs are a separate system from です; three lessons because it reuses U9's polite frame |
| U11 道を聞く | 4 | Follow and give directions | 〜を曲がる, まっすぐ/右/左/角/信号, 〜の先/手前/隣, どのへん | 角, 信号, まっすぐ, 右, 左, 曲がる, 隣, 近く | U10 | The hardest listening task in the course; needs existence verbs and place に from U10 |
| Q2 | 1 | Cumulative gate | — | — | U1–U11 | Half the course, and the point where travel-task vocabulary is broad enough to test without cueing |
| U12 こまったとき | 3 | Report a problem and ask for help | 〜なくした, 〜がない, 〜を忘れた, 〜んですが as softener | 落とす, 忘れる, なくす, 壊れる, 届く, 病院, 財布, 携帯 | U6, U10 | Problems are where repair and permission collide; placed after both so it reuses rather than reteaches |
| U13 予定と誘い | 3 | Make, accept and decline plans | 〜ませんか/〜ましょう, から, つもり/予定, plain + と思います | 予定, つもり, 暇, 忙しい, 一緒に, ちょっと | U4, U5 | First unit producing plain form inside a sentence — the bridge to casual speech |
| U14 経験と比較 | 3 | Talk about what you've done and compare | 〜たことがある, 〜たほうがいい, より/のほうが/一番 | 経験, 一度, 二度, より, 一番, 好き | U13 | Experience talk is the follow-up question a visitor actually gets, and it needs past plain form |
| U15 くだけた話し方 | 4 | Understand casual speech, keep a conversation going | Plain form, contractions for recognition (てる, ちゃう, じゃん, とく, なきゃ), かな/でしょ, あいづち, そうなんですか, 〜んですね | あいづち set, そう, たしかに, なるほど, へえ, じゃあ | U13, U14 | Last new-grammar unit: casual forms only make sense once plain form is producible. Converts a decade of anime listening into usable comprehension |
| Q3 | 1 | Whole-course gate = February benchmark | — | — | All | Final checkpoint before consolidation, in the format he will be measured in |

### Which lesson lands in which week

Units straddle weeks, so the week is not the unit.

| Wk | Lessons |
|---|---|
| 1 | U1 L1–L4 |
| 2 | U2 L1–L3, U3 L1 |
| 3 | U3 L2–L5 |
| 4 | U4 L1–L4 |
| 5 | U5 L1–L4 |
| 6 | U5 L5, U6 L1–L3 |
| 7 | U6 L4, **Q1**, U7 L1–L2 |
| 8 | U7 L3–L4, U8 L1–L2 |
| 9 | U8 L3–L4, U9 L1–L2 |
| 10 | U9 L3–L4, U10 L1–L2 |
| 11 | U10 L3, U11 L1–L3 |
| 12 | U11 L4, **Q2**, U12 L1–L2 |
| 13 | U12 L3, U13 L1–L3 |
| 14 | U14 L1–L3, U15 L1 |
| 15 | U15 L2–L4, **Q3** |

### Per-lesson jobs

| Unit | Lessons |
|---|---|
| U1 | L1 です/じゃない, dropping 私は · L2 は vs が, も · L3 か questions · L4 ききかえし (すみません, もう一度, ゆっくり, わかりません) |
| U2 | L1 すき/きらい + が · L2 ほしい + が vs を · L3 は vs が with すき; とても/あまり/ぜんぜん |
| U3 | L1 verb groups + ます stem · L2 ません/ました/ませんでした · L3 に (time) vs で (place) · L4 を · L5 stem generation on unseen verbs |
| U4 | L1 たい · L2 ない vs ません · L3 を vs が with たい/ほしい · L4 ましょう/ましょうか |
| U5 | L1 て-form rules · L2 て-form sound changes (行って/来て/して, gemination) · L3 〜ている · L4 〜てください · L5 を vs に |
| U6 | L1 question words + か · L2 〜んですか · L3 〜って何ですか; 日本語で何と言いますか · L4 repair scenario with three breakdowns |
| U7 | L1 に/で/から/まで · L2 乗る/降りる/乗り換える · L3 どのくらい/かかります · L4 transport scenario |
| U8 | L1 これ/それ/あれ/どれ; この/その/あの · L2 いくら + the numbers lesson · L3 をください; 〜はありますか · L4 shopping scenario + counters |
| U9 | L1 お〜をください; 〜はありますか · L2 注文 (定食, 大盛り, 持ち帰り) · L3 service keigo, recognition only · L4 restaurant scenario with the アレルギー follow-up |
| U10 | L1 あります/います + に/が · L2 てもいいですか; ないでください · L3 hotel scenario (予約, 荷物を預ける, 〜ています) |
| U11 | L1 〜があります + place に · L2 まっすぐ/右/左/角/信号 · L3 〜を曲がる; 〜の先/手前/隣 · L4 directions scenario + 何分かかりますか |
| U12 | L1 〜なくした; 〜がない · L2 〜んですが · L3 problem scenario (落としました, 届いていますか) |
| U13 | L1 〜ませんか/〜ましょう; から · L2 つもり/予定; plain + と思います · L3 invitation scenario, accept and decline |
| U14 | L1 〜たことがある · L2 〜たほうがいい · L3 より/のほうが/一番 + experience scenario |
| U15 | L1 plain form; かな/でしょ · L2 contractions, recognition first · L3 あいづち; そうなんですか; 〜んですね · L4 six-turn scenario with an unexpected follow-up |

### Naming and order

There is no manifest file and no `order` field, and none is needed: lesson and step
order is YAML order inside a unit file. **Unit order is the sorted filename order**
(`load.go` sorts the collected files, and the TUI walks units in that slice), so files
must be named to sort as the plan sequences them:

```
content/unit-01-jikoshoukai.yaml
content/unit-02-suki.yaml
...
content/unit-15-kudaketa.yaml
```

Two digits is the whole mechanism; without it `unit-10` sorts before `unit-2` and the
learner's first lesson becomes week 10's. Step IDs keep the existing convention from
`unit-b.yaml`: lesson-scoped, e.g. `u3-l1-2`.

## 8. Continuous tracks

Not units, because each is a technique rather than a topic.

| Track | From | What it is | Why not a unit |
|---|---|---|---|
| Question building | W1, owned by U1/U6/U15 | Every unit contributes ≥2 S5 items building questions he has not seen | Required from week one; a unit would delay it to week 6 |
| Repair toolkit | W1 (four phrases), U6 (full) | すみません, もう一度, ゆっくり, わかりません in U1 L4; the rest in U6 | Used in every scenario, so it must precede them |
| Listening volume | W1 | Unseen clips at rising difficulty, answered by meaning | Time on task, not content. **Blocked on P1** |
| Reading recognition | W1 | Katakana, then the ~150 kanji these units use, recognition only | The trip needs signs and menus; lessons would crowd out speaking |
| Same-session retry | W1 | A missed production item re-queues later for no credit | Loop behaviour, not content; needs audit P0-2 |

## 9. Consolidation (weeks 16–19)

No new grammar, no new vocabulary.

| Wk | Focus |
|---|---|
| 16 | Failed-quiz circuit: the concepts Q1–Q3 exposed, through new stimuli only. Two long scenarios |
| 17 | One travel task per day as a scenario circuit: transport, shopping, restaurant, hotel, directions, problems — each with a follow-up he has not seen |
| 18 | Unscripted conversation volume with a human partner where possible; in-app, the longest scenarios in the course. Monthly benchmark 4 |
| 19 | Numbers and prices drill (sharpest failure mode in noise), repair drill, one full trip simulation. No new material in the last 5 days |

## 10. Monthly benchmark

15 minutes, last day of each month (November–February; March is the trip). Same
procedure each month so numbers compare.

1. **Listening, 10 clips, answered by meaning.** Unseen, natural speed, tiered 1–10.
   Score: correct out of 10. **Blocked on P1** — until then a human plays them from
   outside the app and the result is logged by hand.
2. **Two-minute monologue**, same fixed theme each month. Record, transcribe, then
   score from the transcript: clauses per 2 minutes; distinct verb forms used
   (ます, ない, た, て, ている, たい, ことがある); errors per clause, split into
   particle errors and everything else.
3. **Five-turn unscripted roleplay** with a human partner, five prompts rotated
   across months, all drawn from the course's units. Score: turns survived before
   breakdown, and whether he repaired it.

Month to month: clause count and turn count should rise, errors per clause should
fall, form count should rise in course order. A month where errors per clause rise is
not automatically bad if clauses doubled — report both. The benchmark exists to tell
us whether §3's pace rules fire, not to produce a grade.

**Not measurable in-app:** the unscripted roleplay needs an LLM partner (out of scope
this phase) or a human. Stated, not solved.

## 11. Testing the anchor hypothesis

Each lesson is marked `anchored: true` when its anchors are lines he has plausibly
heard in anime. `SCHEMA-PROPOSALS.md` P5 adds the field; until then the marks live in
this document.

After ~16 lessons (roughly half anchored), compare delayed-check accuracy,
first-attempt production accuracy on S3 items, and S4 error-naming success between
anchored and unanchored lessons covering the same concepts. If the gap is under
roughly 10 percentage points, the hypothesis is doing no work and anchors stay
motivational with no claim attached. Reported with numbers, not impressions.

## 12. Blocked and assumed

- **Listening comprehension (P1).** The comprehension track is audio-first by meaning
  and the schema cannot express it: `listening` is dictation-only, and an `audio`
  field on an `interpretation` step lints clean and plays nothing. Every listening
  item here assumes P1. Without it the comprehension track is reading — the wrong
  skill — and benchmark item 1 stays manual.
- **Teaching text (P3).** `content/teach/<lesson-id>.md` is safe to write today
  (the loader reads only `.yaml`/`.yml`); rendering it needs P3. The one-off "how this
  app works" walkthrough is not a lesson — every step kind mints evidence, and
  rustlings' ungraded intro has no equivalent in this schema. It belongs as a `teach`
  document shown on first launch.
- **Retry and reveal after a miss (audit P0-2).** S4 and S8 assume a miss shows the
  accepted answer and re-queues the item. Without it a lesson is a quiz.
- **Concept sharing across units (P2).** Particles and stems recur in ten units each;
  without P2 the coverage report and review schedule fragment, and "fully covered"
  cannot be demonstrated.
- **Irodori alignment.** Units map to Irodori's A1–A2 can-do topics from knowledge of
  the series, not the book. Topic level I stand behind; lesson level I do not claim.

## 13. Where the brief and the code disagree

- The brief expects ~60–65 lessons over ~15–16 weeks before consolidation. This plan
  is 60 lessons over 15 weeks plus 4 consolidation weeks inside a 19-week window. The
  brief's own pace rule plus ~130 hours of available time means the schedule needs
  slack; holding 65 lessons would remove it.
- The brief lists a vocabulary-card kind as an expected proposal. One change covers
  both vocabulary and listening comprehension, so no new kind is proposed — see
  `SCHEMA-PROPOSALS.md` §3.
- The brief's lesson template assumes 6–10 comprehension items per lesson. At 60
  lessons that is 360+ audio items; the coverage floor (3 comprehension + 3 production
  per concept) is what I author to, and the template's count holds only if P1 lands.
- Rustlings' structure is borrowed; its execution is not. See §2.
