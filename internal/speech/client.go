package speech

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// HTTPClient talks to the local whisper.cpp and Kokoro containers over loopback.
type HTTPClient struct {
	STTURL string
	TTSURL string
	HTTP   *http.Client
}

// NewHTTPClient targets the compose stack's fixed endpoints.
func NewHTTPClient() *HTTPClient {
	return &HTTPClient{
		STTURL: "http://" + STTAddr + "/inference",
		TTSURL: "http://" + TTSAddr + "/v1/audio/speech",
		HTTP:   &http.Client{Timeout: 90 * time.Second},
	}
}

type whisperSegment struct {
	AvgLogprob   float64 `json:"avg_logprob"`
	NoSpeechProb float64 `json:"no_speech_prob"`
}

type whisperResponse struct {
	Text     string           `json:"text"`
	Segments []whisperSegment `json:"segments"`
}

// Transcribe sends recorded audio to whisper.cpp and reports what it heard plus how
// confident it was about hearing it.
func (c *HTTPClient) Transcribe(ctx context.Context, wav []byte, language string) (Transcription, error) {
	var body bytes.Buffer
	form := multipart.NewWriter(&body)

	part, err := form.CreateFormFile("file", "audio.wav")
	if err != nil {
		return Transcription{}, err
	}
	if _, err := part.Write(wav); err != nil {
		return Transcription{}, err
	}
	// verbose_json is requested because it carries the confidence fields.
	for _, field := range [][2]string{
		{"language", language},
		{"response_format", "verbose_json"},
	} {
		if err := form.WriteField(field[0], field[1]); err != nil {
			return Transcription{}, err
		}
	}
	if err := form.Close(); err != nil {
		return Transcription{}, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.STTURL, &body)
	if err != nil {
		return Transcription{}, err
	}
	request.Header.Set("Content-Type", form.FormDataContentType())

	response, err := c.HTTP.Do(request)
	if err != nil {
		return Transcription{}, fmt.Errorf("transcribe: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return Transcription{}, fmt.Errorf("transcribe: %s: %s", response.Status, bytes.TrimSpace(message))
	}

	var parsed whisperResponse
	if err := json.NewDecoder(response.Body).Decode(&parsed); err != nil {
		return Transcription{}, fmt.Errorf("transcribe: decode: %w", err)
	}

	result := Transcription{Text: parsed.Text}
	if len(parsed.Segments) > 0 {
		result.Confidence = parsed.Segments[0].AvgLogprob
		result.NoSpeechProb = parsed.Segments[0].NoSpeechProb
	}
	result.Ambiguous = Ambiguity(result)
	return result, nil
}

// Synthesize renders text to WAV audio.
func (c *HTTPClient) Synthesize(ctx context.Context, text, language, voice string) ([]byte, error) {
	if voice == "" {
		voice = "jf_alpha"
	}
	payload, err := json.Marshal(map[string]any{
		"input":           text,
		"voice":           voice,
		"response_format": "wav",
	})
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TTSURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.HTTP.Do(request)
	if err != nil {
		return nil, fmt.Errorf("synthesize: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return nil, fmt.Errorf("synthesize: %s: %s", response.Status, bytes.TrimSpace(message))
	}
	return io.ReadAll(response.Body)
}

// Mock is a controllable client for tests. It records what the app asked for so
// tests can assert on behaviour rather than on audio.
type Mock struct {
	Transcription Transcription
	TranscribeErr error
	Audio         []byte
	SynthErr      error

	Transcribed []string
	Synthesized []string
}

// Transcribe returns the configured transcription.
func (m *Mock) Transcribe(_ context.Context, wav []byte, language string) (Transcription, error) {
	m.Transcribed = append(m.Transcribed, language)
	if m.TranscribeErr != nil {
		return Transcription{}, m.TranscribeErr
	}
	return m.Transcription, nil
}

// Synthesize returns the configured audio.
func (m *Mock) Synthesize(_ context.Context, text, language, voice string) ([]byte, error) {
	m.Synthesized = append(m.Synthesized, text)
	if m.SynthErr != nil {
		return nil, m.SynthErr
	}
	if m.Audio != nil {
		return m.Audio, nil
	}
	return []byte("RIFF....WAVE"), nil
}
