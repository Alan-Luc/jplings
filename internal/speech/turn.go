package speech

import "context"

// Turn is one spoken attempt's confirmation state (R4).
//
// Every spoken response is shown back to the learner before it is graded. Denying a
// transcript re-records the turn and mints no evidence; after two consecutive denials
// the turn offers a typed-romaji fallback or a no-evidence advance, so one consistently
// mis-transcribed utterance can never deadlock progress.
type Turn struct {
	Denials int
}

type Decision struct {
	// Grade means the confirmed transcript becomes the graded text.
	Grade bool
	// OfferFallback means show the typed-romaji / no-evidence-advance choices.
	OfferFallback bool
}

func (t *Turn) Confirm() Decision {
	return Decision{Grade: true}
}

// Deny rejects the transcript: re-record, and mint nothing.
func (t *Turn) Deny() Decision {
	t.Denials++
	return Decision{OfferFallback: t.Denials >= 2}
}

type SpeakResult struct {
	// Degraded means audio could not be produced and the written prompt was revealed.
	Degraded bool
}

// Speak synthesizes and plays a line, retrying once. If audio still fails the session
// reveals the written prompt instead of stalling (S1.4 TTS-down degradation).
func Speak(
	ctx context.Context,
	client Client,
	text, language, voice string,
	play func([]byte) error,
) (SpeakResult, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		audio, err := client.Synthesize(ctx, text, language, voice)
		if err != nil {
			lastErr = err
			continue
		}
		if err := play(audio); err != nil {
			lastErr = err
			continue
		}
		return SpeakResult{}, nil
	}
	// Degradation is not a session failure: the written prompt carries the exercise.
	return SpeakResult{Degraded: true}, lastErr
}
