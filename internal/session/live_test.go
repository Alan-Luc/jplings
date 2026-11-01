//go:build live

package session

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jplings/internal/content"
	"jplings/internal/convo"
	"jplings/internal/profile"
	"jplings/internal/speech"
)

func TestLiveVerticalSlice(t *testing.T) {
	units, issues, err := content.Load(filepath.Join("..", "..", "content"))
	if err != nil {
		t.Fatalf("load content: %v", err)
	}
	if len(issues) > 0 {
		t.Fatalf("content has issues: %v", issues)
	}
	if len(units) == 0 {
		t.Fatal("no content loaded")
	}

	store, err := profile.Open(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	runner := &Runner{
		Store:  store,
		Client: speech.NewHTTPClient(),
		Now:    time.Now,
	}

	ctx := context.Background()
	var steps, ungraded int

	for _, unit := range units {
		for _, lesson := range unit.Lessons {
			for i := range lesson.Steps {
				step := &lesson.Steps[i]
				steps++

				if step.Kind == content.Conversation {
					conversation := convo.New(step.Scenario)
					for !conversation.Done() {
						turn, ok := conversation.Current()
						if ok && turn.Speaker == "learner" {
							answer := turn.Accept[0].Kanji
							if answer == "" {
								answer = turn.Accept[0].Kana
							}
							if got := conversation.Respond(answer); got.Verdict != convo.Matched {
								t.Fatalf("%s: accepted answer %q did not match", step.ID, answer)
							}
						}
						conversation.Advance()
					}
					continue
				}

				answer := answerFor(t, step)

				if step.Kind == content.Speaking && step.Rubric == "" {
					source := step.Accept[0].Kanji
					if source == "" {
						source = step.Accept[0].Kana
					}
					audio, err := runner.Client.Synthesize(ctx, source, "ja", "jf_alpha")
					if err != nil {
						t.Fatalf("%s: synthesize: %v", step.ID, err)
					}
					if hasSpeech, _, err := speech.HasSpeech(audio, speech.SilenceFloor); err != nil || !hasSpeech {
						t.Fatalf("%s: synthesized audio failed the energy gate: %v", step.ID, err)
					}
					heard, err := runner.Client.Transcribe(ctx, audio, "ja")
					if err != nil {
						t.Fatalf("%s: transcribe: %v", step.ID, err)
					}
					answer = heard.Text
				}

				result, err := runner.Submit(ctx, step, answer)
				if err != nil {
					t.Fatalf("%s: submit: %v", step.ID, err)
				}
				if step.Kind == content.Speaking && step.Rubric != "" {
					if !result.Ungraded {
						t.Fatalf("%s: expected an ungraded save with no evaluator, got %+v", step.ID, result)
					}
					ungraded++
					continue
				}
				if !result.Outcome.Correct {
					t.Fatalf("%s: expected credit for %q, got %q", step.ID, answer, result.Outcome.Explanation)
				}
				if result.Ungraded {
					ungraded++
				}
			}
		}
	}

	if steps == 0 {
		t.Fatal("no steps were exercised")
	}
	t.Logf("exercised %d steps (%d saved ungraded)", steps, ungraded)

	for _, unit := range units {
		for _, concept := range unit.Concepts {
			mastered, missing, err := store.Mastered(concept.ID)
			if err != nil {
				t.Fatalf("Mastered(%s): %v", concept.ID, err)
			}
			if mastered {
				t.Fatalf("%s must not be mastered without retention evidence", concept.ID)
			}
			if len(missing) == 0 {
				t.Fatalf("%s reported mastery with nothing missing", concept.ID)
			}
		}
	}

	retention, err := store.DueRetention(time.Now().Add(72 * time.Hour))
	if err != nil {
		t.Fatalf("DueRetention: %v", err)
	}
	if len(retention) == 0 {
		t.Fatal("expected delayed retention checks to be scheduled")
	}
	t.Logf("scheduled %d retention checks", len(retention))
}

func answerFor(t *testing.T, step *content.Step) string {
	t.Helper()
	switch step.Kind {
	case content.Interpretation:
		return step.AcceptMeanings[0]
	case content.Speaking:
		if step.Rubric != "" {
			return "すしをたべたいです"
		}
		return step.Accept[0].Romaji
	default:
		if len(step.Accept) == 0 {
			t.Fatalf("%s has no accepted answers", step.ID)
		}
		return step.Accept[0].Romaji
	}
}

var _ = strings.TrimSpace
