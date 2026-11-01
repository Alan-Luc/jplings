package schedule

import (
	"testing"
	"time"
)

var now = time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)

func TestFirstGoodReviewSchedulesOneDay(t *testing.T) {
	card := Update(Initial("step-c", "c", now), Good, now)
	if card.IntervalDays != 1 {
		t.Fatalf("interval = %d, want 1", card.IntervalDays)
	}
	if !card.Due.Equal(now.AddDate(0, 0, 1)) {
		t.Fatalf("due = %v", card.Due)
	}
	if card.Reps != 1 {
		t.Fatalf("reps = %d, want 1", card.Reps)
	}
}

func TestSecondGoodReviewSchedulesSixDays(t *testing.T) {
	card := Update(Initial("step-c", "c", now), Good, now)
	card = Update(card, Good, now.AddDate(0, 0, 1))
	if card.IntervalDays != 6 {
		t.Fatalf("interval = %d, want 6", card.IntervalDays)
	}
}

func TestAgainResetsProgress(t *testing.T) {
	card := Update(Initial("step-c", "c", now), Good, now)
	card = Update(card, Good, now)
	card = Update(card, Again, now)
	if card.Reps != 0 {
		t.Fatalf("reps = %d, want 0", card.Reps)
	}
	if card.IntervalDays != 1 {
		t.Fatalf("interval = %d, want 1", card.IntervalDays)
	}
}

func TestEaseNeverDropsBelowFloor(t *testing.T) {
	card := Initial("step-c", "c", now)
	for i := 0; i < 50; i++ {
		card = Update(card, Again, now)
	}
	if card.Ease < 1.3 {
		t.Fatalf("ease = %v, want >= 1.3", card.Ease)
	}
}
