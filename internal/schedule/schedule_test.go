package schedule

import (
	"testing"
	"time"
)

var now = time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)

func TestFirstGoodReviewSchedulesOneDay(t *testing.T) {
	card := Review(Initial("c", now), Good, now)
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
	card := Review(Initial("c", now), Good, now)
	card = Review(card, Good, now.AddDate(0, 0, 1))
	if card.IntervalDays != 6 {
		t.Fatalf("interval = %d, want 6", card.IntervalDays)
	}
}

// A wrong answer resets the streak, which is the whole point of an honest schedule.
func TestAgainResetsProgress(t *testing.T) {
	card := Review(Initial("c", now), Good, now)
	card = Review(card, Good, now)
	card = Review(card, Again, now)
	if card.Reps != 0 {
		t.Fatalf("reps = %d, want 0", card.Reps)
	}
	if card.IntervalDays != 1 {
		t.Fatalf("interval = %d, want 1", card.IntervalDays)
	}
}

func TestEaseNeverDropsBelowFloor(t *testing.T) {
	card := Initial("c", now)
	for i := 0; i < 50; i++ {
		card = Review(card, Again, now)
	}
	if card.Ease < 1.3 {
		t.Fatalf("ease = %v, want >= 1.3", card.Ease)
	}
}

func TestDueFiltersByClock(t *testing.T) {
	cards := []Card{
		{ConceptID: "past", Due: now.Add(-time.Hour)},
		{ConceptID: "future", Due: now.Add(time.Hour)},
	}
	due := Due(cards, now)
	if len(due) != 1 || due[0].ConceptID != "past" {
		t.Fatalf("due = %+v, want only the past card", due)
	}
}

// The retention check must be a different day than the teaching session.
func TestRetentionDelayLandsInsideThePlanWindow(t *testing.T) {
	due := RetentionDue(now)
	if due.Before(now.Add(48*time.Hour)) || due.After(now.Add(72*time.Hour)) {
		t.Fatalf("retention due %v is outside the 48-72h window", due)
	}
}
