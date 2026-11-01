package content_test

import (
	"path/filepath"
	"testing"

	"jplings/internal/content"
	"jplings/internal/convo"
)

func TestConversationTurnsMatchAcceptedAnswers(t *testing.T) {
	units, issues, err := content.Load(filepath.Join("..", "..", "content"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("content issues: %v", issues)
	}

	for _, unit := range units {
		for _, lesson := range unit.Lessons {
			for _, step := range lesson.Steps {
				if step.Kind != content.Conversation {
					continue
				}
				for i, turn := range step.Scenario.Turns {
					if turn.Speaker != "learner" {
						continue
					}
					for _, accepted := range turn.Accept {
						for _, answer := range []string{accepted.Kanji, accepted.Kana, accepted.Romaji} {
							if answer == "" {
								continue
							}
							conversation := convo.New(step.Scenario)
							for j := 0; j < i; j++ {
								conversation.Advance()
							}
							if got := conversation.Respond(answer).Verdict; got != convo.Matched {
								t.Fatalf("%s turn %d rejected %q", step.ID, i, answer)
							}
						}
					}
				}
			}
		}
	}
}
