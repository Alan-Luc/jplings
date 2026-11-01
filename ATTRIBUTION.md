# Attribution

jplings curriculum data is authored in-house except where a step declares a
`source:` block. Those steps use material from the corpora below, and the
attribution travels with the step so credit is visible wherever the content is read.

The downloaded corpora themselves are **not** committed. `content/corpus/` is
gitignored and reproduced with `jplings content import`.

## JMdict — vocabulary, readings, kanji forms

- Licence: **CC BY-SA 4.0**
- Copyright: James William Breen and the Electronic Dictionary Research and
  Development Group (EDRDG)
- Source: https://www.edrdg.org/wiki/index.php/JMdict-EDICT_Dictionary_Project

Used for: kana readings, kanji forms, part of speech, and English glosses of
individual words. jplings does not redistribute the dictionary; it records which
entry a step used by `ent_seq`.

Obligations this project accepts:

1. **Attribution** — the EDGRG and the JMdict project are credited here and in each
   step that uses dictionary data.
2. **Share-alike** — any redistributed derivative of JMdict data stays under
   CC BY-SA 4.0. Curriculum files containing imported dictionary data inherit that
   licence for the imported portion; authored explanations do not.
3. **Keeping current** — the licence requires a procedure for updating the data.
   That procedure is: re-run `jplings content import` periodically (monthly is the
   licence's own example). It re-downloads JMdict and Tatoeba and rewrites the
   corpus exports, so readings and glosses track upstream.

## Tatoeba — example sentences and translations

- Licence: **CC BY 2.0 FR**
- Copyright: Tatoeba contributors
- Source: https://tatoeba.org — exports at https://downloads.tatoeba.org/exports/

Used for: authentic Japanese sentences with human English translations, so
listening and reading exercises are not trained solely on synthetic material.

Each imported sentence carries its Tatoeba sentence ID. Where a step quotes one, the
step's `source.id` is that ID.

Nothing from Tatoeba is redistributed in bulk here; sentences enter the curriculum
only when an authored step cites them.

## Audio

Authentic listening audio comes from Tatoeba contributors whose clip licence permits
reuse. Audio licences are per contributor, so a clip is used only when its licence is
explicitly CC BY or CC0, and the step records `audio_source`, `audio_license` and
`audio_attribution` (the content schema rejects partial provenance).

No such clip has been sourced yet; current listening audio is synthesized locally by
Kokoro 82M, which is Apache-2.0 and needs no attribution.

## Tae Kim's Guide to Japanese Grammar

- Licence: **CC BY-NC-SA 3.0**
- Source: https://guidetojapanese.org/

Used as the grammar progression spine only (D9). Explanations in this curriculum are
written in-house; no text is copied. Non-commercial use only, which matches this
project's personal scope. Any redistribution of derived explanatory material must
honour BY-NC-SA.
