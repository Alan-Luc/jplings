package corpus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"jplings/internal/kana"
	"jplings/internal/speech"
)

// Morpheme is one analysed token from the reading service.
type Morpheme struct {
	String string `json:"string"`
	Read   string `json:"read"`
	Pron   string `json:"pron"`
	Pos    string `json:"pos"`
}

// Reading is the kana spelling and romaji pronunciation of a sentence.
type Reading struct {
	Kana   string
	Romaji string
}

// Reader asks the local speech service to analyse sentence text. It reads
// orthographic kana from each morpheme's reading and romaji from its pronunciation,
// because those differ for particles: は reads ハ but is pronounced ワ.
type Reader struct {
	URL  string
	HTTP *http.Client
}

// NewReader targets the TTS container's reading endpoint.
func NewReader() *Reader {
	return &Reader{
		URL:  "http://" + speech.TTSAddr + "/reading",
		HTTP: &http.Client{Timeout: 30 * time.Second},
	}
}

// Read analyses one sentence. The service must be running; a missing analysis is an
// error rather than a silently empty reading.
func (r *Reader) Read(ctx context.Context, text string) (Reading, error) {
	payload, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return Reading{}, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, r.URL, bytes.NewReader(payload))
	if err != nil {
		return Reading{}, err
	}
	request.Header.Set("Content-Type", "application/json")

	client := r.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return Reading{}, fmt.Errorf("reading service: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 256))
		return Reading{}, fmt.Errorf("reading service: %s: %s", response.Status, bytes.TrimSpace(message))
	}

	var parsed struct {
		Morphemes []Morpheme `json:"morphemes"`
	}
	if err := json.NewDecoder(response.Body).Decode(&parsed); err != nil {
		return Reading{}, fmt.Errorf("reading service: %w", err)
	}
	return BuildReading(parsed.Morphemes), nil
}

// BuildReading assembles kana and romaji from analysed morphemes. Punctuation keeps
// its original form, since the analyser normalises it in the reading field.
func BuildReading(morphemes []Morpheme) Reading {
	var kanaText strings.Builder
	var words []string

	for _, morpheme := range morphemes {
		if morpheme.Pos == "記号" {
			kanaText.WriteString(morpheme.String)
			continue
		}
		kanaText.WriteString(kana.ToHiragana(onlyKatakana(morpheme.Read)))
		if romaji := kana.ToRomaji(onlyKatakana(morpheme.Pron)); romaji != "" {
			words = append(words, romaji)
		}
	}
	return Reading{Kana: kanaText.String(), Romaji: strings.Join(words, " ")}
}

func onlyKatakana(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 0x30A1 && r <= 0x30F6:
			return r
		case r == 'ー':
			return r
		default:
			return -1
		}
	}, s)
}

// Enrich attaches kana and romaji to each sentence. It fails loudly rather than
// exporting sentences without readings, which cannot be authored into exercises.
func Enrich(ctx context.Context, reader *Reader, sentences []Sentence) ([]Sentence, error) {
	enriched := make([]Sentence, 0, len(sentences))
	for _, sentence := range sentences {
		reading, err := reader.Read(ctx, sentence.Japanese)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", sentence.Japanese, err)
		}
		sentence.Kana = reading.Kana
		sentence.Romaji = reading.Romaji
		enriched = append(enriched, sentence)
	}
	return enriched, nil
}
