package grade

import (
	"fmt"
	"strings"

	"jplings/internal/content"
	"jplings/internal/normalize"
	"jplings/internal/romaji"
)

type Outcome struct {
	Correct     bool
	Close       bool
	OffTarget   bool
	Explanation string
}

func Written(step *content.Step, answer string) Outcome {
	if out, ok := offTarget(step, answer); ok {
		return out
	}

	verdict, reason := romaji.Compare(answer, textVariants(step.Accept))
	if matchesExact(answer, step.Accept) || verdict == romaji.Exact {
		return Outcome{Correct: true, Explanation: step.Explanation}
	}
	if verdict == romaji.Close {
		return Outcome{
			Close: true,
			Explanation: fmt.Sprintf(
				"Close, but %s changes the word — %q is not %q. %s",
				reason, strings.TrimSpace(answer), step.Accept[0].Romaji, step.Explanation,
			),
		}
	}
	return Outcome{
		Explanation: fmt.Sprintf("Not the expected answer. %s", step.Explanation),
	}
}

func Meaning(step *content.Step, answer string) Outcome {
	got := normalize.Surface(answer)
	if got == "" {
		return Outcome{Explanation: "No answer given. " + step.Explanation}
	}
	for _, want := range step.AcceptMeanings {
		if got == normalize.Surface(want) {
			return Outcome{Correct: true, Explanation: step.Explanation}
		}
	}
	if len(step.AcceptMeanings) == 0 {
		return Outcome{Explanation: "That is not the expected meaning. " + step.Explanation}
	}
	return Outcome{
		Explanation: fmt.Sprintf(
			"That is not the meaning here. Expected something like %q. %s",
			step.AcceptMeanings[0], step.Explanation,
		),
	}
}

func Spoken(step *content.Step, transcript string) Outcome {
	if out, ok := offTargetSpeech(step, transcript); ok {
		return out
	}

	got := normalize.Surface(transcript)

	if suffix := requiredSuffix(step.Accept); suffix != "" && !strings.Contains(got, suffix) {
		return Outcome{
			Explanation: fmt.Sprintf(
				"The number was heard, but the counter is missing — %q needs %q. %s",
				strings.TrimSpace(transcript), suffix, step.Explanation,
			),
		}
	}

	if matchesExact(transcript, step.Accept) {
		return Outcome{Correct: true, Explanation: step.Explanation}
	}

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
		if matchesExact(answer, []content.Text{candidate.Text}) {
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
	got := normalize.Surface(transcript)
	for _, candidate := range step.OffTarget {
		for _, surface := range surfaces(candidate.Text) {
			if got == normalize.Surface(surface) {
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

func matchesExact(answer string, texts []content.Text) bool {
	got := normalize.Surface(answer)
	for _, want := range textVariants(texts) {
		if got == normalize.Surface(want) {
			return true
		}
	}
	return false
}

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

func isNumeralKanji(r rune) bool {
	return strings.ContainsRune("一二三四五六七八九十百千万億零", r)
}
