// Package schedule implements SM-2-style spaced review plus the separate delayed
// retention queue (D6). The clock is always passed in, so retention gates are
// testable without waiting wall-clock time.
package schedule

import (
	"math"
	"time"
)

// Quality is the learner's outcome on a review, on a 0-2 scale matching the rest of
// the app: wrong, understandable but flawed, good.
type Quality int

const (
	Again Quality = iota
	Hard
	Good
)

// Card is one concept's review state.
type Card struct {
	ConceptID    string
	Ease         float64
	IntervalDays int
	Reps         int
	Due          time.Time
}

// Initial is a concept's starting review state, due immediately.
func Initial(conceptID string, now time.Time) Card {
	return Card{ConceptID: conceptID, Ease: 2.5, IntervalDays: 0, Reps: 0, Due: now}
}

// Review applies SM-2 to a card and returns its new state.
func Review(card Card, quality Quality, now time.Time) Card {
	if card.Ease == 0 {
		card.Ease = 2.5
	}

	// SM-2's 0-5 scale, mapped from this app's three outcomes.
	sm2 := map[Quality]float64{Again: 0, Hard: 3, Good: 5}[quality]

	card.Ease = math.Max(1.3, card.Ease+0.1-(5-sm2)*(0.08+(5-sm2)*0.02))

	if sm2 < 3 {
		card.Reps = 0
		card.IntervalDays = 1
		card.Due = now.AddDate(0, 0, card.IntervalDays)
		return card
	}

	switch card.Reps {
	case 0:
		card.IntervalDays = 1
	case 1:
		card.IntervalDays = 6
	default:
		card.IntervalDays = int(math.Round(float64(card.IntervalDays) * card.Ease))
	}
	card.Reps++
	card.Due = now.AddDate(0, 0, card.IntervalDays)
	return card
}

// RetentionDelay is how long after first clearing production and generalization the
// delayed retention check is scheduled. The plan's window is 48-72 hours; the middle
// of that window is used so the check lands on a different day without drifting out.
const RetentionDelay = 60 * time.Hour

// RetentionDue is when a concept's different-context retention check becomes due.
func RetentionDue(now time.Time) time.Time { return now.Add(RetentionDelay) }

// Due returns the concepts whose review is due at now, earliest first.
func Due(cards []Card, now time.Time) []Card {
	var due []Card
	for _, card := range cards {
		if !card.Due.After(now) {
			due = append(due, card)
		}
	}
	return due
}
