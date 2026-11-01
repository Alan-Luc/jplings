package speech

import "context"

type Transcription struct {
	Text         string
	Confidence   float64
	NoSpeechProb float64
	Ambiguous    bool
}

type Client interface {
	Transcribe(ctx context.Context, wav []byte, language string) (Transcription, error)
	Synthesize(ctx context.Context, text, language, voice string) ([]byte, error)
}

const ConfidenceFloor = -0.8

func Ambiguity(t Transcription) bool {
	return t.Confidence < ConfidenceFloor
}
