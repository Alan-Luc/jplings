// Package speech is the app's only door to STT and TTS (R4, D8).
//
// The curriculum engine never learns which model answers: it sees a small interface,
// so either service can be replaced without touching exercises. Transcription
// success is never treated as pronunciation proof.
package speech

import "context"

// Transcription is what the app heard, plus what it knows about its own uncertainty.
type Transcription struct {
	Text string
	// Confidence is Whisper's average log probability for the segment. It is a
	// secondary signal only: silence produces fluent text at near-perfect confidence.
	Confidence float64
	// NoSpeechProb is Whisper's own guess that the clip had no speech. Also secondary.
	NoSpeechProb float64
	// Ambiguous marks a transcript the learner must confirm before it is graded.
	Ambiguous bool
}

// Client is the stable speech interface. Implementations must not leak model details.
type Client interface {
	// Transcribe interprets recorded audio. The caller decides what to do with
	// low confidence; this method only reports it.
	Transcribe(ctx context.Context, wav []byte, language string) (Transcription, error)
	// Synthesize returns playable WAV audio for text.
	Synthesize(ctx context.Context, text, language, voice string) ([]byte, error)
}

// ConfidenceFloor is the segment log probability below which a transcript is flagged
// ambiguous. It is deliberately not a no-speech test — see HasSpeech for that.
const ConfidenceFloor = -0.8

// Ambiguity decides whether a transcript needs explicit learner confirmation. Every
// spoken response is confirmed regardless; this only marks the shaky ones in the UI.
func Ambiguity(t Transcription) bool {
	return t.Confidence < ConfidenceFloor
}
