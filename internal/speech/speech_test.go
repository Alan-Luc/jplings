package speech

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func tone(samples int, amplitude float64) []byte {
	buf := make([]int16, samples)
	for i := range buf {
		buf[i] = int16(math.Sin(float64(i)/8.0) * amplitude * 32767)
	}
	return EncodeWAV(buf, 24000)
}

func silence(samples int) []byte {
	return EncodeWAV(make([]int16, samples), 24000)
}

// The plan's central speech-safety claim, pinned as a test: silence must be caught
// from the waveform, because the recogniser will happily invent text for it.
func TestHasSpeechRejectsSilence(t *testing.T) {
	hasSpeech, rms, err := HasSpeech(silence(24000), SilenceFloor)
	if err != nil {
		t.Fatalf("HasSpeech: %v", err)
	}
	if hasSpeech {
		t.Fatalf("silence must not count as speech (rms=%v)", rms)
	}
}

func TestHasSpeechAcceptsSpeechLevelEnergy(t *testing.T) {
	hasSpeech, rms, err := HasSpeech(tone(24000, 0.2), SilenceFloor)
	if err != nil {
		t.Fatalf("HasSpeech: %v", err)
	}
	if !hasSpeech {
		t.Fatalf("speech-level audio must count as speech (rms=%v)", rms)
	}
}

func TestHasSpeechRejectsNonPCM(t *testing.T) {
	if _, _, err := HasSpeech([]byte("not a wav at all"), SilenceFloor); !errors.Is(err, ErrNotPCM) {
		t.Fatalf("expected ErrNotPCM, got %v", err)
	}
}

// Two consecutive denials must offer the fallback rather than trapping the learner.
func TestTwoDenialsOfferFallback(t *testing.T) {
	var turn Turn

	if decision := turn.Deny(); decision.Grade || decision.OfferFallback {
		t.Fatalf("first denial should only re-record, got %+v", decision)
	}
	if decision := turn.Deny(); !decision.OfferFallback {
		t.Fatal("second consecutive denial must offer the fallback")
	}
}

func TestConfirmedTranscriptIsGraded(t *testing.T) {
	var turn Turn
	if decision := turn.Confirm(); !decision.Grade {
		t.Fatal("a confirmed transcript becomes the graded text")
	}
}

func TestSpeakRetriesOnceThenDegrades(t *testing.T) {
	mock := &Mock{SynthErr: errors.New("tts down")}
	played := 0

	result, err := Speak(context.Background(), mock, "水をください", "ja", "jf_alpha",
		func([]byte) error { played++; return nil })

	if !result.Degraded {
		t.Fatal("expected the session to degrade to the written prompt")
	}
	if err == nil {
		t.Fatal("the underlying failure should still be reported for logging")
	}
	if len(mock.Synthesized) != 2 {
		t.Fatalf("expected exactly one retry, got %d attempts", len(mock.Synthesized))
	}
	if played != 0 {
		t.Fatalf("nothing should play when synthesis fails, played=%d", played)
	}
}

func TestSpeakDegradesWhenPlaybackFails(t *testing.T) {
	mock := &Mock{Audio: []byte("RIFF....WAVE")}

	result, err := Speak(context.Background(), mock, "水をください", "ja", "jf_alpha",
		func([]byte) error { return errors.New("mpv missing") })

	if !result.Degraded || err == nil {
		t.Fatalf("playback failure should degrade, got %+v err=%v", result, err)
	}
}

func TestSpeakSucceedsWithoutDegrading(t *testing.T) {
	mock := &Mock{Audio: []byte("RIFF....WAVE")}

	result, err := Speak(context.Background(), mock, "水をください", "ja", "jf_alpha",
		func([]byte) error { return nil })

	if result.Degraded || err != nil {
		t.Fatalf("expected clean playback, got %+v err=%v", result, err)
	}
}

// The HTTP client must read confidence out of the real response shape, so the
// low-confidence branch is driven by the server rather than assumed.
func TestHTTPClientTranscribeReadsConfidence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("expected multipart form: %v", err)
		}
		if got := r.FormValue("response_format"); got != "verbose_json" {
			t.Errorf("response_format = %q, want verbose_json", got)
		}
		if got := r.FormValue("language"); got != "ja" {
			t.Errorf("language = %q, want ja", got)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"text": "水をください\n",
			"segments": []map[string]any{
				{"avg_logprob": -0.09, "no_speech_prob": 0.000003},
			},
		})
	}))
	defer server.Close()

	client := &HTTPClient{STTURL: server.URL, HTTP: server.Client()}
	got, err := client.Transcribe(context.Background(), []byte("audio"), "ja")
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if strings.TrimSpace(got.Text) != "水をください" {
		t.Fatalf("text = %q", got.Text)
	}
	if math.Abs(got.Confidence-(-0.09)) > 1e-9 {
		t.Fatalf("confidence = %v", got.Confidence)
	}
	if got.Ambiguous {
		t.Fatal("a confident transcript should not be flagged ambiguous")
	}
}

func TestHTTPClientFlagsLowConfidenceAsAmbiguous(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"text":     "よくわからない",
			"segments": []map[string]any{{"avg_logprob": -1.6}},
		})
	}))
	defer server.Close()

	client := &HTTPClient{STTURL: server.URL, HTTP: server.Client()}
	got, err := client.Transcribe(context.Background(), []byte("audio"), "ja")
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if !got.Ambiguous {
		t.Fatal("low confidence must be flagged ambiguous so the learner confirms carefully")
	}
}

func TestHTTPClientReportsServerErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model not loaded", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := &HTTPClient{STTURL: server.URL, TTSURL: server.URL, HTTP: server.Client()}
	if _, err := client.Transcribe(context.Background(), []byte("audio"), "ja"); err == nil {
		t.Fatal("expected an error for a non-200 response")
	}
	if _, err := client.Synthesize(context.Background(), "こんにちは", "ja", "jf_alpha"); err == nil {
		t.Fatal("expected an error for a non-200 TTS response")
	}
}

func TestHTTPClientSynthesizeReturnsAudio(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("expected JSON body: %v", err)
		}
		if payload["input"] != "こんにちは" {
			t.Errorf("input = %v", payload["input"])
		}
		w.Write([]byte("RIFFaudio"))
	}))
	defer server.Close()

	client := &HTTPClient{TTSURL: server.URL, HTTP: server.Client()}
	audio, err := client.Synthesize(context.Background(), "こんにちは", "ja", "jf_alpha")
	if err != nil {
		t.Fatalf("Synthesize: %v", err)
	}
	if string(audio) != "RIFFaudio" {
		t.Fatalf("audio = %q", audio)
	}
}
