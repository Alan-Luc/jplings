package session

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"jplings/internal/content"
	"jplings/internal/eval"
	"jplings/internal/profile"
	"jplings/internal/schedule"
)

var now = time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)

type fakeEvaluator struct {
	rubric eval.Rubric
	err    error
}

func (f fakeEvaluator) Evaluate(context.Context, string, string) (eval.Rubric, error) {
	return f.rubric, f.err
}

func newRunner(t *testing.T, evaluator eval.Evaluator) (*Runner, *profile.Store) {
	t.Helper()
	store, err := profile.Open(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	return &Runner{
		Store:     store,
		Evaluator: evaluator,
		Now:       func() time.Time { return now },
	}, store
}

func productionStep(id, stimulus string) *content.Step {
	return &content.Step{
		ID:       id,
		Kind:     content.Construction,
		Concepts: []string{"c1"},
		Prompt:   "Say it.",
		Accept: []content.Text{
			{Kanji: "水をください", Kana: "みずをください", Romaji: "mizu o kudasai"},
		},
		Explanation: "を marks the object.",
	}
}

func TestCorrectAnswerMintsProductionEvidence(t *testing.T) {
	runner, store := newRunner(t, nil)

	result, err := runner.Submit(context.Background(), productionStep("s1", "s1"), "mizu o kudasai")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if !result.Outcome.Correct || !result.MintedEvidence {
		t.Fatalf("expected credit, got %+v", result)
	}

	evidence, err := store.Evidence("c1")
	if err != nil {
		t.Fatalf("Evidence: %v", err)
	}
	if len(evidence) != 1 || evidence[0].Dimension != profile.Production {
		t.Fatalf("evidence = %+v", evidence)
	}
}

func TestRepeatingTheSameStimulusMintsNoGeneralization(t *testing.T) {
	runner, store := newRunner(t, nil)
	step := productionStep("s1", "s1")

	for i := 0; i < 2; i++ {
		if _, err := runner.Submit(context.Background(), step, "mizu o kudasai"); err != nil {
			t.Fatalf("Submit: %v", err)
		}
	}

	evidence, _ := store.Evidence("c1")
	for _, e := range evidence {
		if e.Dimension == profile.Generalization {
			t.Fatalf("repeating one stimulus must not count as generalization: %+v", evidence)
		}
	}
	if can, _ := store.CanSkip("c1"); can {
		t.Fatal("one repeated stimulus must not satisfy the skip gate")
	}
}

func TestDifferentStimulusMintsGeneralizationAndSchedulesRetention(t *testing.T) {
	runner, store := newRunner(t, nil)

	if _, err := runner.Submit(context.Background(), productionStep("s1", "s1"), "mizu o kudasai"); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if _, err := runner.Submit(context.Background(), productionStep("s2", "s2"), "mizu o kudasai"); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	mastered, missing, err := store.Mastered("c1")
	if err != nil {
		t.Fatalf("Mastered: %v", err)
	}
	if mastered {
		t.Fatal("mastery must still be blocked without retention evidence")
	}
	if len(missing) != 1 || missing[0] != profile.Retention {
		t.Fatalf("missing = %v, want only retention", missing)
	}

	cards, err := store.Reviews()
	if err != nil {
		t.Fatalf("Reviews: %v", err)
	}
	var retention *schedule.Card
	for i := range cards {
		if cards[i].Kind == schedule.KindRetention {
			retention = &cards[i]
		}
	}
	if retention == nil {
		t.Fatalf("expected a scheduled retention check, got %+v", cards)
	}
	nowValue := runner.Now()
	due := retention.Due.Sub(nowValue)
	if due < 48*time.Hour || due > 72*time.Hour {
		t.Fatalf("retention scheduled %v out, want 48-72h", due)
	}

	if dueNow, _ := store.DueRetention(nowValue); len(dueNow) != 0 {
		t.Fatalf("a retention check 60h out must not be due now: %+v", dueNow)
	}
}

func TestCloseAnswerMintsNothingAndLogsAnError(t *testing.T) {
	runner, store := newRunner(t, nil)

	result, err := runner.Submit(context.Background(), productionStep("s1", "s1"), "mizu o kudasaii")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if result.MintedEvidence {
		t.Fatal("a near miss must not mint evidence")
	}
	if result.Outcome.Explanation == "" {
		t.Fatal("feedback must explain the miss")
	}

	if evidence, _ := store.Evidence("c1"); len(evidence) != 0 {
		t.Fatalf("no evidence should be recorded, got %+v", evidence)
	}
	errors, _ := store.Errors("c1")
	if len(errors) != 1 {
		t.Fatalf("expected the miss to enter the error log, got %+v", errors)
	}
}

func TestEvaluatorFailureSavesUngraded(t *testing.T) {
	step := &content.Step{
		ID: "free", Kind: content.Speaking, Concepts: []string{"c1"},
		Prompt: "Say something you want to eat.", Rubric: "judge it",
	}

	tests := []struct {
		name      string
		evaluator eval.Evaluator
	}{
		{"no evaluator", nil},
		{"api error", fakeEvaluator{err: errors.New("503 service unavailable")}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner, store := newRunner(t, tc.evaluator)

			result, err := runner.Submit(context.Background(), step, "すしをたべたいです")
			if err != nil {
				t.Fatalf("Submit: %v", err)
			}
			if !result.Ungraded {
				t.Fatal("expected the response to be saved ungraded")
			}
			if result.MintedEvidence {
				t.Fatal("an ungraded response must never mint evidence")
			}

			pending, err := store.PendingToRetry()
			if err != nil || len(pending) != 1 {
				t.Fatalf("expected one pending response, got %+v err=%v", pending, err)
			}
			if pending[0].Response != "すしをたべたいです" {
				t.Fatalf("pending response = %q", pending[0].Response)
			}
		})
	}
}

func TestEvaluatorSuccessIsProvisionalLLMEvidence(t *testing.T) {
	step := &content.Step{
		ID: "free", Kind: content.Speaking, Concepts: []string{"c1"},
		Prompt: "Say something you want to eat.", Rubric: "judge it",
	}
	runner, store := newRunner(t, fakeEvaluator{
		rubric: eval.Rubric{Meaning: 2, Grammar: 1, Naturalness: 1, Register: 2, Feedback: "Good, watch the particle."},
	})

	result, err := runner.Submit(context.Background(), step, "すしをたべたいです")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if !result.AIGraded || !result.MintedEvidence {
		t.Fatalf("expected AI-graded credit, got %+v", result)
	}

	evidence, _ := store.Evidence("c1")
	if len(evidence) != 1 || evidence[0].Source != "llm" {
		t.Fatalf("evidence = %+v", evidence)
	}
	if mastered, _, _ := store.Mastered("c1"); mastered {
		t.Fatal("provisional LLM evidence must not clear mastery")
	}
}

func TestEvaluatorRejectsBrokenGrammarEvenWithGoodMeaning(t *testing.T) {
	step := &content.Step{
		ID: "free", Kind: content.Speaking, Concepts: []string{"c1"},
		Prompt: "Say something you want to eat.", Rubric: "judge it",
	}
	runner, _ := newRunner(t, fakeEvaluator{
		rubric: eval.Rubric{Meaning: 2, Grammar: 0, Naturalness: 0, Register: 1, Feedback: "Meaning clear, grammar broken."},
	})

	result, err := runner.Submit(context.Background(), step, "すし たべたい です が")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if result.Outcome.Correct {
		t.Fatal("broken grammar must not earn credit even when the meaning is clear")
	}
}

func TestSpeakWithoutClientDegrades(t *testing.T) {
	runner, _ := newRunner(t, nil)
	result, err := runner.Speak(context.Background(), "水をください", func([]byte) error { return nil })
	if err != nil {
		t.Fatalf("Speak: %v", err)
	}
	if !result.Degraded {
		t.Fatal("no speech client must degrade to the written prompt, not fail")
	}
}

func TestReviewScheduledOnCorrectAnswer(t *testing.T) {
	runner, store := newRunner(t, nil)
	if _, err := runner.Submit(context.Background(), productionStep("s1", "s1"), "mizu o kudasai"); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	cards, err := store.Reviews()
	if err != nil || len(cards) == 0 {
		t.Fatalf("expected a scheduled review, got %+v err=%v", cards, err)
	}
	if cards[0].Kind != schedule.KindReview {
		t.Fatalf("kind = %q, want review", cards[0].Kind)
	}
}
