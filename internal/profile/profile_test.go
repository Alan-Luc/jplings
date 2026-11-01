package profile

import (
	"database/sql"
	"fmt"
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

func recordSource(t *testing.T, store *Store, dimension, stimulus, source string) {
	t.Helper()
	err := store.Record(Evidence{
		ConceptID: "c1", Dimension: dimension, Correct: true,
		StimulusID: stimulus, Source: source, At: now,
	})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
}

func TestMasteredRequiresProductionOnTwoStimuliAndRetention(t *testing.T) {
	store := open(t)

	mastered, missing, err := store.Mastered("c1")
	if err != nil {
		t.Fatalf("Mastered: %v", err)
	}
	if mastered {
		t.Fatal("an untouched concept must not be mastered")
	}
	if len(missing) != 2 {
		t.Fatalf("missing = %v, want production and retention", missing)
	}

	record(t, store, Production, "s1")
	record(t, store, Production, "s2")
	if mastered, missing, _ := store.Mastered("c1"); mastered {
		t.Fatalf("mastery without retention evidence must not clear; missing=%v", missing)
	}
	if can, _ := store.CanSkip("c1"); !can {
		t.Fatal("skipping must stay strictly weaker than mastery")
	}

	record(t, store, Retention, "s3")
	mastered, missing, err = store.Mastered("c1")
	if err != nil {
		t.Fatalf("Mastered: %v", err)
	}
	if !mastered {
		t.Fatalf("expected mastery, missing=%v", missing)
	}
}

func TestLLMEvidenceNeedsANonLLMCredit(t *testing.T) {
	store := open(t)
	recordSource(t, store, Production, "s1", "llm")
	recordSource(t, store, Production, "s2", "llm")
	record(t, store, Retention, "r")

	mastered, missing, _ := store.Mastered("c1")
	if mastered {
		t.Fatal("LLM-only production must not satisfy mastery")
	}
	if len(missing) != 1 || missing[0] != Production {
		t.Fatalf("missing = %v, want production", missing)
	}
	if can, _ := store.CanSkip("c1"); can {
		t.Fatal("LLM-only production must not unlock skipping")
	}

	recordSource(t, store, Production, "s1", "typed")
	if can, _ := store.CanSkip("c1"); !can {
		t.Fatal("one non-LLM credit among two stimuli should unlock skipping")
	}
	if mastered, _, _ := store.Mastered("c1"); !mastered {
		t.Fatal("expected mastery once production and retention are satisfied")
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
	card := schedule.Update(schedule.Initial("s1", "c1", now), schedule.Good, now)
	if err := store.EnsureCard(card); err != nil {
		t.Fatalf("EnsureCard: %v", err)
	}
	cards, err := store.Reviews()
	if err != nil || len(cards) != 1 {
		t.Fatalf("Reviews: %v err=%v", cards, err)
	}
	if cards[0].StepID != "s1" || cards[0].IntervalDays != 1 {
		t.Fatalf("card = %+v, want step s1 at a 1 day interval", cards[0])
	}
}

func TestEnsureCardKeepsExistingProgress(t *testing.T) {
	store := open(t)
	card := schedule.Update(schedule.Initial("s1", "c1", now), schedule.Good, now)
	if err := store.EnsureCard(card); err != nil {
		t.Fatalf("EnsureCard: %v", err)
	}
	if err := store.EnsureCard(schedule.Initial("s1", "c1", now.Add(time.Hour))); err != nil {
		t.Fatalf("EnsureCard again: %v", err)
	}

	cards, err := store.Reviews()
	if err != nil || len(cards) != 1 {
		t.Fatalf("Reviews: %v err=%v", cards, err)
	}
	if cards[0].Reps != 1 || cards[0].IntervalDays != 1 {
		t.Fatalf("a repeated success rebuilt the card: %+v", cards[0])
	}

	advanced := schedule.Update(cards[0], schedule.Good, now.AddDate(0, 0, 1))
	if err := store.UpdateCard(advanced); err != nil {
		t.Fatalf("UpdateCard: %v", err)
	}
	cards, _ = store.Reviews()
	if cards[0].Reps != 2 || cards[0].IntervalDays != 6 {
		t.Fatalf("UpdateCard did not advance the card: %+v", cards[0])
	}
}

func TestDueCardsCapsTheBlock(t *testing.T) {
	store := open(t)
	for i := 0; i < 13; i++ {
		stepID := fmt.Sprintf("s%02d", i)
		if err := store.EnsureCard(schedule.Initial(stepID, "c1", now.AddDate(0, 0, -2).Add(-time.Duration(i)*time.Hour))); err != nil {
			t.Fatalf("EnsureCard: %v", err)
		}
	}

	due, err := store.DueCards(now, 12)
	if err != nil {
		t.Fatalf("DueCards: %v", err)
	}
	if len(due) != 12 {
		t.Fatalf("due = %d, want the 12-card cap", len(due))
	}
}

func TestOpenMigratesOldConceptKeyedReviews(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	old, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open old db: %v", err)
	}
	_, err = old.Exec(`CREATE TABLE reviews (
	  concept_id TEXT NOT NULL,
	  kind TEXT NOT NULL DEFAULT 'review',
	  ease REAL NOT NULL,
	  interval_days INTEGER NOT NULL,
	  reps INTEGER NOT NULL,
	  due_at INTEGER NOT NULL,
	  PRIMARY KEY (concept_id, kind)
	)`)
	if err != nil {
		t.Fatalf("create old reviews: %v", err)
	}
	if _, err := old.Exec(`INSERT INTO reviews VALUES ('c1', 'retention', 2.5, 0, 0, 0)`); err != nil {
		t.Fatalf("seed old review: %v", err)
	}
	if err := old.Close(); err != nil {
		t.Fatalf("close old db: %v", err)
	}

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open old shape: %v", err)
	}
	defer store.Close()

	if err := store.EnsureCard(schedule.Initial("s1", "c1", now)); err != nil {
		t.Fatalf("EnsureCard after migration: %v", err)
	}
	var stepRows int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM reviews WHERE step_id = 's1'`).Scan(&stepRows); err != nil {
		t.Fatalf("count step rows: %v", err)
	}
	if stepRows != 1 {
		t.Fatalf("the migrated table must store one step-keyed row, got %d", stepRows)
	}
	var leftovers int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM reviews WHERE concept_id = 'c1' AND kind = 'retention'`).Scan(&leftovers); err != nil {
		t.Fatalf("count old rows: %v", err)
	}
	if leftovers != 0 {
		t.Fatalf("the concept-keyed rows are dropped deliberately, got %d left", leftovers)
	}
	cards, err := store.Reviews()
	if err != nil || len(cards) != 1 || cards[0].StepID != "s1" {
		t.Fatalf("migrated reviews = %+v err=%v", cards, err)
	}

	var version int
	if err := store.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatalf("user_version: %v", err)
	}
	if version != 1 {
		t.Fatalf("user_version = %d, want 1", version)
	}
}
