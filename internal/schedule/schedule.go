package schedule

import (
	"math"
	"time"
)

type Quality int

const (
	Again Quality = iota
	Hard
	Good
)

const (
	KindReview    = "review"
	KindRetention = "retention"
)

type Card struct {
	ConceptID    string
	Ease         float64
	IntervalDays int
	Reps         int
	Due          time.Time
	Kind         string
}

func Initial(conceptID string, now time.Time) Card {
	return Card{ConceptID: conceptID, Ease: 2.5, IntervalDays: 0, Reps: 0, Due: now, Kind: KindReview}
}

func Review(card Card, quality Quality, now time.Time) Card {
	if card.Ease == 0 {
		card.Ease = 2.5
	}

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

const RetentionDelay = 60 * time.Hour

func RetentionDue(now time.Time) time.Time { return now.Add(RetentionDelay) }

func RetentionCard(conceptID string, now time.Time) Card {
	return Card{ConceptID: conceptID, Ease: 2.5, Due: RetentionDue(now), Kind: KindRetention}
}

func Due(cards []Card, now time.Time) []Card {
	var due []Card
	for _, card := range cards {
		if !card.Due.After(now) {
			due = append(due, card)
		}
	}
	return due
}
