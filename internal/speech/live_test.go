//go:build live

package speech

import (
	"context"
	"strings"
	"testing"
)

func TestLiveTranscribeAndSynthesize(t *testing.T) {
	client := NewHTTPClient()
	ctx := context.Background()

	audio, err := client.Synthesize(ctx, "水をください。", "ja", "jf_alpha")
	if err != nil {
		t.Fatalf("Synthesize against the real container: %v", err)
	}
	if len(audio) == 0 {
		t.Fatal("TTS returned no audio")
	}

	hasSpeech, rms, err := HasSpeech(audio, SilenceFloor)
	if err != nil {
		t.Fatalf("HasSpeech on synthesized audio: %v", err)
	}
	if !hasSpeech {
		t.Fatalf("synthesized audio has no energy (rms=%v)", rms)
	}

	got, err := client.Transcribe(ctx, audio, "ja")
	if err != nil {
		t.Fatalf("Transcribe against the real container: %v", err)
	}
	if strings.TrimSpace(got.Text) != "水をください" {
		t.Fatalf("round trip = %q, want 水をください", got.Text)
	}
	if got.Ambiguous {
		t.Fatalf("a clean round trip should not be flagged ambiguous (confidence=%v)", got.Confidence)
	}
}

func TestLiveSilenceIsRejectedBeforeRecognition(t *testing.T) {
	silent := EncodeWAV(make([]int16, 24000), 24000)

	hasSpeech, rms, err := HasSpeech(silent, SilenceFloor)
	if err != nil {
		t.Fatalf("HasSpeech: %v", err)
	}
	if hasSpeech {
		t.Fatalf("digital silence must not pass the energy gate (rms=%v)", rms)
	}

	client := NewHTTPClient()
	if junk, err := client.Transcribe(context.Background(), silent, "ja"); err == nil {
		t.Logf("whisper on silence returned %q (confidence %.3f, no_speech %.6f)",
			strings.TrimSpace(junk.Text), junk.Confidence, junk.NoSpeechProb)
	}
}

func TestLiveJapaneseCounterSpeech(t *testing.T) {
	client := NewHTTPClient()
	ctx := context.Background()

	audio, err := client.Synthesize(ctx, "八百円です。", "ja", "jf_alpha")
	if err != nil {
		t.Fatalf("Synthesize: %v", err)
	}
	got, err := client.Transcribe(ctx, audio, "ja")
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if !strings.Contains(got.Text, "800") && !strings.Contains(got.Text, "八百") {
		t.Fatalf("expected 800 yen back, got %q", got.Text)
	}
}
