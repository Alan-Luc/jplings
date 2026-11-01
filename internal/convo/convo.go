// Package convo interprets scripted conversations (D4).
//
// Scenarios are authored, not generated: predictable, testable, and honest. Matching
// tolerates the variations a learner will actually produce — dropped particles,
// politeness prefixes, です/ます — but a tolerated turn is reported with a hint and
// mints no correct evidence.
package convo

import (
	"fmt"
	"strings"

	"jplings/internal/content"
)

// Verdict is how a learner's turn was understood.
type Verdict int

const (
	// NoMatch is not a recognizable response to this turn.
	NoMatch Verdict = iota
	// Matched is an accepted response and earns credit.
	Matched
	// Tolerated was understood but dropped or added something. No credit.
	Tolerated
)

// Response is the outcome of one learner turn.
type Response struct {
	Verdict Verdict
	// Hint explains a tolerated turn, naming what was dropped.
	Hint string
}

var particles = []string{"を", "は", "が", "に", "で", "へ", "と", "も", "の"}

// Conversation walks a scripted scenario.
type Conversation struct {
	scenario *content.Scenario
	turn     int
}

// New starts a scenario at its first turn.
func New(scenario *content.Scenario) *Conversation {
	return &Conversation{scenario: scenario}
}

// Current returns the turn awaiting action, and whether the scenario has one.
func (c *Conversation) Current() (content.Turn, bool) {
	if c.scenario == nil || c.turn >= len(c.scenario.Turns) {
		return content.Turn{}, false
	}
	return c.scenario.Turns[c.turn], true
}

// Advance moves to the next turn.
func (c *Conversation) Advance() { c.turn++ }

// Done reports whether the scenario has finished.
func (c *Conversation) Done() bool {
	return c.scenario == nil || c.turn >= len(c.scenario.Turns)
}

// Respond interprets a learner's answer against the current learner turn.
func (c *Conversation) Respond(text string) Response {
	turn, ok := c.Current()
	if !ok || turn.Speaker != "learner" {
		return Response{Verdict: NoMatch}
	}

	got := normalize(text)
	for _, accepted := range turn.Accept {
		for _, surface := range surfaces(accepted) {
			if got == normalize(surface) {
				return Response{Verdict: Matched}
			}
		}
	}

	// A dropped particle or prefix still communicates; report it and mint nothing.
	for _, accepted := range turn.Accept {
		for _, surface := range surfaces(accepted) {
			if hint, ok := tolerate(got, normalize(surface)); ok {
				return Response{Verdict: Tolerated, Hint: hint}
			}
		}
	}
	return Response{Verdict: NoMatch}
}

// tolerate reports whether got is base with a particle or politeness prefix removed,
// and names what went missing so the feedback can teach.
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

	// です/ます can be elided in speech without changing what was asked.
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

func normalize(s string) string {
	s = strings.TrimSpace(s)
	s = strings.NewReplacer("。", "", "、", "", " ", "", "　", "", "！", "", "？", "", "?", "").Replace(s)
	return s
}
