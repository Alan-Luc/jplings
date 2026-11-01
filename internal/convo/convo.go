package convo

import (
	"fmt"
	"strings"

	"jplings/internal/content"
	"jplings/internal/normalize"
)

type Verdict int

const (
	NoMatch Verdict = iota
	Matched
	Tolerated
)

type Response struct {
	Verdict Verdict
	Hint    string
}

var particles = []string{"を", "は", "が", "に", "で", "へ", "と", "も", "の"}

type Conversation struct {
	scenario *content.Scenario
	turn     int
}

func New(scenario *content.Scenario) *Conversation {
	return &Conversation{scenario: scenario}
}

func (c *Conversation) Current() (content.Turn, bool) {
	if c.scenario == nil || c.turn >= len(c.scenario.Turns) {
		return content.Turn{}, false
	}
	return c.scenario.Turns[c.turn], true
}

func (c *Conversation) Advance() { c.turn++ }

func (c *Conversation) Done() bool {
	return c.scenario == nil || c.turn >= len(c.scenario.Turns)
}

func (c *Conversation) Respond(text string) Response {
	turn, ok := c.Current()
	if !ok || turn.Speaker != "learner" {
		return Response{Verdict: NoMatch}
	}

	got := normalize.Surface(text)
	for _, accepted := range turn.Accept {
		for _, surface := range surfaces(accepted) {
			if got == normalize.Surface(surface) {
				return Response{Verdict: Matched}
			}
		}
	}

	for _, accepted := range turn.Accept {
		for _, surface := range surfaces(accepted) {
			if hint, ok := tolerate(got, normalize.Surface(surface)); ok {
				return Response{Verdict: Tolerated, Hint: hint}
			}
		}
	}
	return Response{Verdict: NoMatch}
}

func tolerate(got, base string) (string, bool) {
	for _, particle := range particles {
		if strings.Contains(base, particle) && strings.ReplaceAll(base, particle, "") == got {
			return fmt.Sprintf("Understood, but %s was dropped — %s marks the role of the word before it.", particle, particle), true
		}
	}

	for _, prefix := range []string{"お", "ご"} {
		trimmed := strings.TrimPrefix(base, prefix)
		if trimmed != base && trimmed == got {
			return fmt.Sprintf("Understood — %s is the polite prefix, so %s is the usual form here.", prefix, base), true
		}
	}

	for _, ending := range []string{"です", "ます"} {
		if strings.HasSuffix(base, ending) && strings.TrimSuffix(base, ending) == got {
			return fmt.Sprintf("Understood, though %s keeps the register polite.", ending), true
		}
	}
	return "", false
}

func surfaces(t content.Text) []string {
	out := []string{t.Kana, t.Romaji}
	if t.Kanji != "" {
		out = append(out, t.Kanji)
	}
	return out
}
