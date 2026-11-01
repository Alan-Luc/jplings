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

func (f fakeEvaluator) Evaluate(context.Context, string, string, string) (eval.Rubric, error) {
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

func TestDifferentStimuliSatisfyProductionButNotRetention(t *testing.T) {
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
	if err != nil || len(cards) != 2 {
		t.Fatalf("expected a card per passed step, got %+v err=%v", cards, err)
	}
	for _, card := range cards {
		if card.Reps != 0 {
			t.Fatalf("a lesson success must create the card unadvanced, got %+v", card)
		}
		if card.Due.Before(now.Add(23 * time.Hour)) {
			t.Fatalf("a new card must be due a day out so retention is a delayed check, got %+v", card)
		}
	}
}

func TestRubricStepsCreateNoReviewCard(t *testing.T) {
	runner, store := newRunner(t, fakeEvaluator{rubric: eval.Rubric{Meaning: 2, Grammar: 2, Naturalness: 2, Register: 2}})
	step := &content.Step{
		ID: "s1", Kind: content.Speaking, Concepts: []string{"c1"},
		Prompt: "Say something you want to eat.", Rubric: "meaning, grammar, naturalness, register",
	}

	result, err := runner.Submit(context.Background(), step, "すしをたべたいです")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if !result.Outcome.Correct {
		t.Fatalf("expected a correct free-form verdict, got %+v", result.Outcome)
	}

	cards, err := store.Reviews()
	if err != nil {
		t.Fatalf("Reviews: %v", err)
	}
	if len(cards) != 0 {
		t.Fatalf("the deterministic graders cannot answer a rubric step, so it must not become a card: %+v", cards)
	}
}

func TestCardAdvancesAcrossSessionsWithoutResetting(t *testing.T) {
	store, err := profile.Open(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	clock := now
	runner := &Runner{Store: store, Now: func() time.Time { return clock }}
	step := productionStep("s1", "s1")
	if _, err := runner.Submit(context.Background(), step, "mizu o kudasai"); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	clock = clock.AddDate(0, 0, 1)

	wantIntervals := []int{1, 6, 16}
	for session := 0; session < 3; session++ {
		due, err := runner.DueCards()
		if err != nil || len(due) != 1 {
			t.Fatalf("session %d due cards = %+v err=%v", session, due, err)
		}
		result, err := runner.Review(due[0], step, "mizu o kudasai", false)
		if err != nil {
			t.Fatalf("session %d Review: %v", session, err)
		}
		if !result.Outcome.Correct || !result.MintedEvidence {
			t.Fatalf("session %d expected retention credit, got %+v", session, result)
		}

		cards, _ := store.Reviews()
		if len(cards) != 1 {
			t.Fatalf("session %d cards = %+v", session, cards)
		}
		if cards[0].Reps != session+1 {
			t.Fatalf("session %d reps = %d, want %d (card reset)", session, cards[0].Reps, session+1)
		}
		if cards[0].IntervalDays != wantIntervals[session] {
			t.Fatalf("session %d interval = %d, want %d", session, cards[0].IntervalDays, wantIntervals[session])
		}
		clock = clock.AddDate(0, 0, cards[0].IntervalDays)
	}
}

func TestReviewingASiblingAdvancesTheServedCard(t *testing.T) {
	runner, store := newRunner(t, nil)
	ctx := context.Background()

	if _, err := runner.Submit(ctx, productionStep("s1", "s1"), "mizu o kudasai"); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	cards, _ := store.Reviews()
	if err := store.UpdateCard(schedule.Update(cards[0], schedule.Good, now)); err != nil {
		t.Fatalf("UpdateCard: %v", err)
	}

	sibling := productionStep("s2", "s2")
	if _, err := runner.Submit(ctx, sibling, "mizu o kudasai"); err != nil {
		t.Fatalf("Submit sibling: %v", err)
	}

	served, _ := store.Reviews()
	var card schedule.Card
	for _, got := range served {
		if got.StepID == "s1" {
			card = got
		}
	}
	if _, err := runner.Review(card, sibling, "mizu o kudasai", false); err != nil {
		t.Fatalf("Review sibling: %v", err)
	}

	served, _ = store.Reviews()
	for _, got := range served {
		switch got.StepID {
		case "s1":
			if got.Reps != 2 || got.IntervalDays != 6 {
				t.Fatalf("the served card did not advance: %+v", got)
			}
		case "s2":
			if got.Reps != 0 {
				t.Fatalf("the sibling's own card must not advance or rebuild: %+v", got)
			}
		}
	}
}

func TestReviewInTheSameSessionMintsNoRetentionEvidence(t *testing.T) {
	runner, store := newRunner(t, nil)
	step := productionStep("s1", "s1")
	if _, err := runner.Submit(context.Background(), step, "mizu o kudasai"); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	cards, _ := store.Reviews()

	result, err := runner.Review(cards[0], step, "mizu o kudasai", true)
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if !result.Outcome.Correct || result.MintedEvidence {
		t.Fatalf("a same-session review must not mint retention evidence, got %+v", result)
	}

	evidence, _ := store.Evidence("c1")
	for _, e := range evidence {
		if e.Dimension == profile.Retention {
			t.Fatalf("same-session review minted retention: %+v", evidence)
		}
	}
	if cards, _ := store.Reviews(); cards[0].Reps != 1 {
		t.Fatalf("the card should still advance when reviewed: %+v", cards[0])
	}
}

func TestPendingRetryGradesAndResolvesTheSavedResponse(t *testing.T) {
	step := &content.Step{
		ID: "free", Kind: content.Speaking, Concepts: []string{"c1"},
		Prompt: "Say something you want to eat.", Rubric: "judge it",
	}
	runner, store := newRunner(t, fakeEvaluator{
		rubric: eval.Rubric{Meaning: 2, Grammar: 1, Naturalness: 1, Register: 2, Feedback: "Good."},
	})
	if _, err := store.EnqueuePending(step.ID, step.Prompt, "すしをたべたいです", now); err != nil {
		t.Fatalf("EnqueuePending: %v", err)
	}

	lookup := func(id string) *content.Step {
		if id == step.ID {
			return step
		}
		return nil
	}
	if err := runner.RetryPending(context.Background(), lookup); err != nil {
		t.Fatalf("RetryPending: %v", err)
	}

	evidence, _ := store.Evidence("c1")
	if len(evidence) != 1 || evidence[0].Source != "llm" {
		t.Fatalf("retry should grade the saved response, got %+v", evidence)
	}
	if retry, _ := store.PendingToRetry(); len(retry) != 0 {
		t.Fatalf("a graded retry must leave the queue, got %+v", retry)
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

func TestVerbatimTargetEarnsNoProductionEvidence(t *testing.T) {
	step := &content.Step{
		ID: "free", Kind: content.Speaking, Concepts: []string{"c1"},
		Prompt: "Ask for water.", Rubric: "judge it",
		Target: &content.Text{Kanji: "水をください", Kana: "みずをください", Romaji: "mizu o kudasai"},
	}
	runner, store := newRunner(t, fakeEvaluator{
		rubric: eval.Rubric{Meaning: 2, Grammar: 2, Naturalness: 2, Register: 2, Feedback: "Good."},
	})

	result, err := runner.Submit(context.Background(), step, "水をください。")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if !result.AIGraded {
		t.Fatal("a verbatim target is still graded")
	}
	if result.MintedEvidence {
		t.Fatal("reproducing the target verbatim must not mint production evidence")
	}
	if evidence, _ := store.Evidence("c1"); len(evidence) != 0 {
		t.Fatalf("verbatim target earned evidence: %+v", evidence)
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

func hearingStep() *content.Step {
	return &content.Step{
		ID:       "s1",
		Kind:     content.Listening,
		Concepts: []string{"c1"},
		Prompt:   "What is being said about the place?",
		Target: &content.Text{
			Kanji: "駅の近くにコンビニがあります", Kana: "えきのちかくにコンビニがあります",
			Romaji: "eki no chikaku ni konbini ga arimasu",
		},
		AcceptMeanings: []string{
			"There is a convenience store near the station.",
			"Near the station there's a convenience store.",
		},
		Explanation: "に marks where the thing exists.",
	}
}

func TestListeningWithMeaningsIsGradedByMeaning(t *testing.T) {
	runner, store := newRunner(t, nil)

	result, err := runner.Submit(context.Background(), hearingStep(), "there is a convenience store near the station")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if !result.Outcome.Correct || !result.MintedEvidence {
		t.Fatalf("a correct meaning must be credited, got %+v", result)
	}

	evidence, err := store.Evidence("c1")
	if err != nil {
		t.Fatalf("Evidence: %v", err)
	}
	if len(evidence) != 1 || evidence[0].Dimension != profile.Comprehension {
		t.Fatalf("listening graded by meaning must credit comprehension, got %+v", evidence)
	}
}

func TestTranscribedJapaneseIsNotAMeaningAnswer(t *testing.T) {
	runner, _ := newRunner(t, nil)

	result, err := runner.Submit(context.Background(), hearingStep(), "eki no chikaku ni konbini ga arimasu")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if result.Outcome.Correct {
		t.Fatalf("the transcription is not the meaning, got %+v", result.Outcome)
	}
	if result.Outcome.Explanation == "" {
		t.Fatal("feedback must never be a bare incorrect")
	}
}

func TestCorrectAnswersCreateOneCardPerStep(t *testing.T) {
	runner, store := newRunner(t, nil)

	for i := 0; i < 2; i++ {
		if _, err := runner.Submit(context.Background(), productionStep("s1", "s1"), "mizu o kudasai"); err != nil {
			t.Fatalf("Submit s1: %v", err)
		}
	}
	if _, err := runner.Submit(context.Background(), productionStep("s2", "s2"), "mizu o kudasai"); err != nil {
		t.Fatalf("Submit s2: %v", err)
	}

	cards, err := store.Reviews()
	if err != nil || len(cards) != 2 {
		t.Fatalf("expected one card per passed step, got %+v err=%v", cards, err)
	}
}

func TestMatchedAcceptCreditsOnlyItsOwnConcepts(t *testing.T) {
	runner, store := newRunner(t, nil)
	step := &content.Step{
		ID:       "s1",
		Kind:     content.Construction,
		Concepts: []string{"polite"},
		Prompt:   "Say \"I do not eat sushi.\"",
		Accept: []content.Text{
			{Kanji: "寿司は食べません", Kana: "すしはたべません", Romaji: "sushi wa tabemasen", Concepts: []string{"wa"}},
			{Kanji: "寿司を食べません", Kana: "すしをたべません", Romaji: "sushi o tabemasen", Concepts: []string{"wo"}},
		},
		Explanation: "は makes sushi the topic; を makes it the object.",
	}

	result, err := runner.Submit(context.Background(), step, "sushi wa tabemasen")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if !result.Outcome.Correct || !result.MintedEvidence {
		t.Fatalf("expected credit, got %+v", result)
	}

	for concept, want := range map[string]int{"wa": 1, "wo": 0, "polite": 0} {
		evidence, err := store.Evidence(concept)
		if err != nil {
			t.Fatalf("Evidence(%s): %v", concept, err)
		}
		if len(evidence) != want {
			t.Fatalf("concept %s evidence = %+v, want %d", concept, evidence, want)
		}
	}
}

func TestUnannotatedAcceptStillCreditsStepConcepts(t *testing.T) {
	runner, store := newRunner(t, nil)

	if _, err := runner.Submit(context.Background(), productionStep("s1", "s1"), "mizu o kudasai"); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	evidence, _ := store.Evidence("c1")
	if len(evidence) != 1 {
		t.Fatalf("an accept without concepts must credit the step concepts, got %+v", evidence)
	}
}

func TestSpokenAcceptCreditsItsOwnConcepts(t *testing.T) {
	runner, store := newRunner(t, nil)
	step := &content.Step{
		ID:       "s1",
		Kind:     content.Speaking,
		Concepts: []string{"price"},
		Prompt:   "Say the price aloud.",
		Accept: []content.Text{
			{Kanji: "六百円", Kana: "ろっぴゃくえん", Romaji: "roppyaku en", Concepts: []string{"rendaku"}},
		},
		Explanation: "ろっぴゃく is the rendaku reading of 六百.",
	}

	if _, err := runner.Submit(context.Background(), step, "六百円"); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if evidence, _ := store.Evidence("rendaku"); len(evidence) != 1 {
		t.Fatalf("the matched spoken accept must credit rendaku, got %+v", evidence)
	}
	if evidence, _ := store.Evidence("price"); len(evidence) != 0 {
		t.Fatalf("the step concept must not be credited when the matched accept names concepts, got %+v", evidence)
	}
}

func TestGuidedAttemptIsExcludedFromMasterySkippingAndRetention(t *testing.T) {
	runner, store := newRunner(t, nil)

	for _, id := range []string{"s1", "s2"} {
		result, err := runner.SubmitGuided(context.Background(), productionStep(id, id), "mizu o kudasai")
		if err != nil {
			t.Fatalf("SubmitGuided(%s): %v", id, err)
		}
		if !result.Outcome.Correct || !result.MintedEvidence {
			t.Fatalf("a guided correct answer should still be recorded, got %+v", result)
		}
	}

	evidence, err := store.Evidence("c1")
	if err != nil || len(evidence) != 2 {
		t.Fatalf("expected two guided attempts, got %+v err=%v", evidence, err)
	}
	for _, e := range evidence {
		if e.Source != "guided" {
			t.Fatalf("guided attempt source = %q, want guided", e.Source)
		}
	}

	if can, _ := store.CanSkip("c1"); can {
		t.Fatal("guided attempts must not unlock skipping")
	}
	mastered, missing, _ := store.Mastered("c1")
	if mastered {
		t.Fatal("guided attempts must not satisfy mastery")
	}
	for _, dimension := range missing {
		if dimension == profile.Production {
			if cards, _ := store.Reviews(); len(cards) != 0 {
				t.Fatalf("guided attempts must not create review cards, got %+v", cards)
			}
			return
		}
	}
	t.Fatal("guided attempts must leave production missing")
}
