package profile

import (
	"path/filepath"
	"testing"
	"time"

	"jplings/internal/schedule"
)

var now = time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)

func open(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func record(t *testing.T, store *Store, dimension string, stimulus string) {
	t.Helper()
	err := store.Record(Evidence{
		ConceptID: "c1", Dimension: dimension, Correct: true,
		StimulusID: stimulus, Source: "typed", At: now,
	})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
}

func TestProfilePersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.db")

	first, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := first.Record(Evidence{ConceptID: "c1", Dimension: Production, Correct: true, At: now}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if err := first.SaveCursor("unit-a", "a-topic", 3, now); err != nil {
		t.Fatalf("SaveCursor: %v", err)
	}
	first.Close()

	second, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer second.Close()

	evidence, err := second.Evidence("c1")
	if err != nil {
		t.Fatalf("Evidence: %v", err)
	}
	if len(evidence) != 1 {
		t.Fatalf("evidence did not survive restart: %+v", evidence)
	}
	unit, lesson, step, ok, err := second.Cursor()
	if err != nil || !ok {
		t.Fatalf("Cursor: %v ok=%v", err, ok)
	}
	if unit != "unit-a" || lesson != "a-topic" || step != 3 {
		t.Fatalf("cursor = %s/%s/%d", unit, lesson, step)
	}
}

func TestMasteredRequiresProductionGeneralizationAndRetention(t *testing.T) {
	store := open(t)

	mastered, missing, err := store.Mastered("c1")
	if err != nil {
		t.Fatalf("Mastered: %v", err)
	}
	if mastered {
		t.Fatal("an untouched concept must not be mastered")
	}
	if len(missing) != 3 {
		t.Fatalf("missing = %v, want all three dimensions", missing)
	}

	record(t, store, Production, "s1")
	record(t, store, Generalization, "s2")
	if mastered, missing, _ := store.Mastered("c1"); mastered {
		t.Fatalf("mastery without retention evidence must not clear; missing=%v", missing)
	}

	record(t, store, Retention, "s3")
	mastered, missing, err = store.Mastered("c1")
	if err != nil {
		t.Fatalf("Mastered: %v", err)
	}
	if !mastered {
		t.Fatalf("expected mastery with all three dimensions, missing=%v", missing)
	}
}

func TestProvisionalLLMEvidenceCannotClearMastery(t *testing.T) {
	store := open(t)
	for _, dimension := range []string{Production, Generalization, Retention} {
		err := store.Record(Evidence{
			ConceptID: "c1", Dimension: dimension, Correct: true,
			StimulusID: dimension, Source: "llm", At: now,
		})
		if err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	if mastered, _, _ := store.Mastered("c1"); mastered {
		t.Fatal("provisional LLM evidence must not satisfy mastery")
	}

	if err := store.SetLLMTrust("c1", Production, "trusted"); err != nil {
		t.Fatalf("SetLLMTrust: %v", err)
	}
	if err := store.SetLLMTrust("c1", Generalization, "trusted"); err != nil {
		t.Fatalf("SetLLMTrust: %v", err)
	}
	if err := store.SetLLMTrust("c1", Retention, "trusted"); err != nil {
		t.Fatalf("SetLLMTrust: %v", err)
	}
	if mastered, _, _ := store.Mastered("c1"); !mastered {
		t.Fatal("trusted LLM evidence should satisfy mastery")
	}
}

func TestSkipGateRequiresTwoDifferentStimuli(t *testing.T) {
	store := open(t)

	if can, _ := store.CanSkip("c1"); can {
		t.Fatal("a concept with no evidence must not be skippable")
	}

	record(t, store, Production, "same")
	if can, _ := store.CanSkip("c1"); can {
		t.Fatal("skipping after one demonstration must not be allowed")
	}

	record(t, store, Production, "same")
	if can, _ := store.CanSkip("c1"); can {
		t.Fatal("two demonstrations of the same stimulus must not count")
	}

	record(t, store, Production, "different")
	can, err := store.CanSkip("c1")
	if err != nil {
		t.Fatalf("CanSkip: %v", err)
	}
	if !can {
		t.Fatal("two different stimuli should allow skipping")
	}
}

func TestFailedEvidenceDoesNotCount(t *testing.T) {
	store := open(t)
	err := store.Record(Evidence{
		ConceptID: "c1", Dimension: Production, Correct: false,
		StimulusID: "s1", At: now,
	})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if can, _ := store.CanSkip("c1"); can {
		t.Fatal("incorrect attempts must not satisfy the skip gate")
	}
}

func TestPendingQueueNeedsManualAfterTwoRetries(t *testing.T) {
	store := open(t)

	id, err := store.EnqueuePending("step-1", "Say something", "water please", now)
	if err != nil {
		t.Fatalf("EnqueuePending: %v", err)
	}

	retry, err := store.PendingToRetry()
	if err != nil || len(retry) != 1 {
		t.Fatalf("expected one retryable item, got %v err=%v", retry, err)
	}

	if err := store.RetryPending(id); err != nil {
		t.Fatalf("RetryPending: %v", err)
	}
	if err := store.RetryPending(id); err != nil {
		t.Fatalf("RetryPending: %v", err)
	}

	if retry, _ := store.PendingToRetry(); len(retry) != 0 {
		t.Fatal("an item that failed twice must not keep retrying automatically")
	}
	manual, err := store.PendingNeedsManual()
	if err != nil || len(manual) != 1 {
		t.Fatalf("expected one manually graded item, got %v err=%v", manual, err)
	}

	if err := store.ResolvePending(id); err != nil {
		t.Fatalf("ResolvePending: %v", err)
	}
	if manual, _ := store.PendingNeedsManual(); len(manual) != 0 {
		t.Fatal("a graded item must leave the queue")
	}
}

func TestBackupProducesVerifiedSnapshotAndPrunes(t *testing.T) {
	store := open(t)
	record(t, store, Production, "s1")

	dir := filepath.Join(t.TempDir(), "backups")
	first, err := store.Backup(dir, 2)
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if err := verify(first); err != nil {
		t.Fatalf("backup snapshot failed integrity check: %v", err)
	}

	restored, err := Open(first)
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	defer restored.Close()
	evidence, err := restored.Evidence("c1")
	if err != nil || len(evidence) != 1 {
		t.Fatalf("restored profile lost evidence: %v err=%v", evidence, err)
	}
}

func TestReviewSchedulingRoundTrip(t *testing.T) {
	store := open(t)
	card := schedule.Review(schedule.Initial("c1", now), schedule.Good, now)
	if err := store.ScheduleReview(card); err != nil {
		t.Fatalf("ScheduleReview: %v", err)
	}
	cards, err := store.Reviews()
	if err != nil || len(cards) != 1 {
		t.Fatalf("Reviews: %v err=%v", cards, err)
	}
	if cards[0].IntervalDays != 1 {
		t.Fatalf("interval = %d, want 1", cards[0].IntervalDays)
	}
}
