// Package grade holds the deterministic graders (R3, D3).
//
// Every grader returns feedback that names what went wrong. None of them awards
// credit for a near miss: a close-only match, a tolerated conversation variant, and
// an off-target-but-valid form all mint no correct evidence.
package grade

import (
	"fmt"
	"strings"

	"jplings/internal/content"
	"jplings/internal/romaji"
)

// Outcome is one grading result.
type Outcome struct {
	// Correct earns credit and may mint evidence.
	Correct bool
	// Close is a near miss differing only in vowel length or gemination. No credit.
	Close bool
	// OffTarget is valid Japanese answering a different question. No credit.
	OffTarget bool
	// Explanation always names the specific issue, never bare correct/incorrect.
	Explanation string
}

// Dimension is the profile evidence dimension an exercise kind can mint.
func Dimension(kind content.Kind) string {
	switch kind {
	case content.Interpretation, content.Listening:
		return "comprehension"
	case content.Construction, content.Transformation, content.Completion,
		content.Speaking, content.Conversation:
		return "production"
	default:
		return ""
	}
}

// Written grades a typed answer for the Japanese-answer kinds. The learner types
// romaji by default, but an IME produces kana or kanji, so those surfaces are
// accepted too (D7).
func Written(step *content.Step, answer string) Outcome {
	if out, ok := offTarget(step, answer); ok {
		return out
	}

	verdict, reason := romaji.Compare(answer, textVariants(step.Accept))
	switch verdict {
	case romaji.Exact:
		return Outcome{Correct: true, Explanation: step.Explanation}
	case romaji.Close:
		return Outcome{
			Close: true,
			Explanation: fmt.Sprintf(
				"Close, but %s changes the word — %q is not %q. %s",
				reason, strings.TrimSpace(answer), step.Accept[0].Romaji, step.Explanation,
			),
		}
	default:
		return Outcome{
			Explanation: fmt.Sprintf("Not the expected answer. %s", step.Explanation),
		}
	}
}

// Meaning grades an interpretation exercise against authored English meanings.
func Meaning(step *content.Step, answer string) Outcome {
	got := normalizeMeaning(answer)
	if got == "" {
		return Outcome{Explanation: "No answer given. " + step.Explanation}
	}
	for _, want := range step.AcceptMeanings {
		if got == normalizeMeaning(want) {
			return Outcome{Correct: true, Explanation: step.Explanation}
		}
	}
	return Outcome{
		Explanation: fmt.Sprintf(
			"That is not the meaning here. Expected something like %q. %s",
			step.AcceptMeanings[0], step.Explanation,
		),
	}
}

// Spoken grades a confirmed speech transcript. Pronunciation quality is never
// scored, but word identity always is: naming a different word is a wrong answer
// with a specific explanation (R4, S1.6).
func Spoken(step *content.Step, transcript string) Outcome {
	if out, ok := offTargetSpeech(step, transcript); ok {
		return out
	}

	got := normalizeJapan(transcript)

	// A required counter may not be dropped: 六百 without 円 is not 六百円.
	if suffix := requiredSuffix(step.Accept); suffix != "" && !strings.Contains(got, suffix) {
		return Outcome{
			Explanation: fmt.Sprintf(
				"The number was heard, but the counter is missing — %q needs %q. %s",
				strings.TrimSpace(transcript), suffix, step.Explanation,
			),
		}
	}

	for _, accepted := range step.Accept {
		for _, surface := range surfaces(accepted) {
			if got == normalizeJapan(surface) {
				return Outcome{Correct: true, Explanation: step.Explanation}
			}
		}
	}

	// Close-but-different still names the distinction rather than a bare refusal.
	verdict, reason := romaji.Compare(transcript, textVariants(step.Accept))
	if verdict == romaji.Close {
		return Outcome{
			Close: true,
			Explanation: fmt.Sprintf(
				"Close, but %s changes the word — that is a different word, not a pronunciation problem. %s",
				reason, step.Explanation,
			),
		}
	}
	return Outcome{
		Explanation: fmt.Sprintf(
			"That is a different word from the target. %s", step.Explanation,
		),
	}
}

func offTarget(step *content.Step, answer string) (Outcome, bool) {
	for _, candidate := range step.OffTarget {
		if verdict, _ := romaji.Compare(answer, surfaces(candidate.Text)); verdict == romaji.Exact {
			return Outcome{
				OffTarget: true,
				Explanation: fmt.Sprintf(
					"That is valid Japanese, but it means %q rather than what the prompt asked. %s",
					candidate.Meaning, step.Explanation,
				),
			}, true
		}
	}
	return Outcome{}, false
}

func offTargetSpeech(step *content.Step, transcript string) (Outcome, bool) {
	got := normalizeJapan(transcript)
	for _, candidate := range step.OffTarget {
		for _, surface := range surfaces(candidate.Text) {
			if got == normalizeJapan(surface) {
				return Outcome{
					OffTarget: true,
					Explanation: fmt.Sprintf(
						"That is valid Japanese, but it means %q rather than what the prompt asked. %s",
						candidate.Meaning, step.Explanation,
					),
				}, true
			}
		}
	}
	return Outcome{}, false
}

// surfaces lists every written surface of a text: romaji, kana, and kanji when present.
func surfaces(t content.Text) []string {
	out := []string{t.Romaji, t.Kana}
	if t.Kanji != "" {
		out = append(out, t.Kanji)
	}
	return out
}

func textVariants(texts []content.Text) []string {
	var out []string
	for _, t := range texts {
		out = append(out, surfaces(t)...)
	}
	return out
}

// requiredSuffix returns the trailing counter in an accepted answer, such as 円.
// It reads the kanji surface because kana spelling cannot separate a numeral from
// its counter without a morphological parser.
func requiredSuffix(accepted []content.Text) string {
	if len(accepted) == 0 || accepted[0].Kanji == "" {
		return ""
	}
	runes := []rune(accepted[0].Kanji)
	i := len(runes)
	for i > 0 && isKanjiRune(runes[i-1]) && !isNumeralKanji(runes[i-1]) {
		i--
	}
	suffix := string(runes[i:])
	if len([]rune(suffix)) > 2 {
		return ""
	}
	return suffix
}

func isKanjiRune(r rune) bool { return r >= 0x4E00 && r <= 0x9FFF }

// isNumeralKanji keeps the numeral part of an amount out of the counter suffix.
func isNumeralKanji(r rune) bool {
	return strings.ContainsRune("一二三四五六七八九十百千万億零", r)
}

func isDigitRune(r rune) bool { return r >= '0' && r <= '9' }

func normalizeMeaning(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimSuffix(s, ".")
	s = strings.NewReplacer(",", " ", "!", "", "?", "", "  ", " ").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

// normalizeJapan folds digit and kanji-numeral spellings of the same amount so
// 600円, 六百円 and ６００円 compare equal, while counters are preserved.
func normalizeJapan(s string) string {
	s = strings.ReplaceAll(s, "　", "")
	s = expandFullwidthDigits(s)
	s = strings.NewReplacer("。", "", "、", "", " ", "", ".", "", "！", "", "？", "").Replace(s)
	return digitsToKanji(s)
}

// digitsToKanji rewrites runs of ASCII digits as kanji numerals.
func digitsToKanji(s string) string {
	var out strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); {
		if !isDigitRune(runes[i]) {
			out.WriteRune(runes[i])
			i++
			continue
		}
		start := i
		for i < len(runes) && isDigitRune(runes[i]) {
			i++
		}
		out.WriteString(numberToKanji(string(runes[start:i])))
	}
	return out.String()
}

func expandFullwidthDigits(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '０' && r <= '９' {
			return '0' + (r - '０')
		}
		return r
	}, s)
}

var kanjiDigits = []string{"", "一", "二", "三", "四", "五", "六", "七", "八", "九"}
var kanjiUnits = []struct {
	value int
	kanji string
}{
	{10000000, "千万"}, {1000000, "百万"}, {100000, "十万"},
	{10000, "万"}, {1000, "千"}, {100, "百"}, {10, "十"},
}

// numberToKanji renders a decimal digit string as a kanji numeral, so that typed
// and spoken amounts share one canonical form.
func numberToKanji(digits string) string {
	digits = strings.TrimLeft(digits, "0")
	if digits == "" {
		return "零"
	}
	value := 0
	for _, r := range digits {
		value = value*10 + int(r-'0')
	}
	return valueToKanji(value)
}

func valueToKanji(value int) string {
	if value == 0 {
		return "零"
	}
	var out strings.Builder
	for _, unit := range kanjiUnits {
		if value < unit.value {
			continue
		}
		count := value / unit.value
		value %= unit.value
		if count > 1 || unit.value >= 10000 {
			out.WriteString(kanjiDigits[count])
		}
		out.WriteString(unit.kanji)
	}
	if value > 0 {
		out.WriteString(kanjiDigits[value])
	}
	return out.String()
}
