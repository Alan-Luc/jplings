package speech

import "context"

type Turn struct {
	Denials int
}

type Decision struct {
	Grade         bool
	OfferFallback bool
}

func (t *Turn) Confirm() Decision {
	return Decision{Grade: true}
}

func (t *Turn) Deny() Decision {
	t.Denials++
	return Decision{OfferFallback: t.Denials >= 2}
}

type SpeakResult struct {
	Degraded bool
}

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
	return SpeakResult{Degraded: true}, lastErr
}
