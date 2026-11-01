// Package eval is the free-form grader (D3).
//
// It exists only because no deterministic grader can score open-ended production.
// Any failure — missing key, non-200, unparseable body, out-of-range score — behaves
// identically: the caller saves the response ungraded for the pending queue. It is
// never auto-passed and never crashes a session.
package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// DefaultModel is pinned so a grading change is a deliberate act, not a drift.
	DefaultModel = "deepseek-chat"
	// DefaultBaseURL is the DeepSeek chat completion endpoint.
	DefaultBaseURL = "https://api.deepseek.com/chat/completions"
)

// Rubric is the four independently-scored dimensions plus feedback.
type Rubric struct {
	Meaning     int    `json:"meaning"`
	Grammar     int    `json:"grammar"`
	Naturalness int    `json:"naturalness"`
	Register    int    `json:"register"`
	Feedback    string `json:"feedback"`
}

// Validate rejects anything the model was not asked to produce, so a malformed
// answer becomes an ungraded save rather than a bogus score.
func (r Rubric) Validate() error {
	for name, score := range map[string]int{
		"meaning": r.Meaning, "grammar": r.Grammar,
		"naturalness": r.Naturalness, "register": r.Register,
	} {
		if score < 0 || score > 2 {
			return fmt.Errorf("%s = %d, want 0, 1 or 2", name, score)
		}
	}
	if r.Feedback == "" {
		return fmt.Errorf("missing feedback")
	}
	return nil
}

// Evaluator grades a free-form response against a prompt.
type Evaluator interface {
	Evaluate(ctx context.Context, prompt, response string) (Rubric, error)
}

// Client calls DeepSeek directly. There is no agent loop and no separate service.
type Client struct {
	APIKey  string
	BaseURL string
	Model   string
	HTTP    *http.Client
}

// NewClient builds a client for the given key.
func NewClient(apiKey string) *Client {
	return &Client{
		APIKey:  apiKey,
		BaseURL: DefaultBaseURL,
		Model:   DefaultModel,
		HTTP:    &http.Client{Timeout: 45 * time.Second},
	}
}

const systemPrompt = `You grade one Japanese learner's spoken response, supplied as a transcript.

Return JSON only, with exactly these keys:
  meaning, grammar, naturalness, register  (integers 0, 1 or 2)
  feedback                                  (one or two sentences, in English)

Scale: 0 = wrong or unintelligible, 1 = understandable but flawed, 2 = good.

Rules:
- Grade the meaning the learner actually expressed, not only the prompt's literal wording.
- A grammatically valid form that says something different is a meaning error even
  when the grammar is correct. Score the dimensions independently.
- Never score pronunciation. You only see text.
- In feedback, name the specific problem. Never reply with a bare "incorrect".`

type chatRequest struct {
	Model          string  `json:"model"`
	Temperature    float64 `json:"temperature"`
	ResponseFormat struct {
		Type string `json:"type"`
	} `json:"response_format"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Evaluate grades one response. Any failure is returned as an error so the caller can
// save it ungraded.
func (c *Client) Evaluate(ctx context.Context, prompt, response string) (Rubric, error) {
	if c.APIKey == "" {
		return Rubric{}, fmt.Errorf("no API key configured")
	}

	request := chatRequest{Model: c.Model, Temperature: 0}
	request.ResponseFormat.Type = "json_object"
	request.Messages = []chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: fmt.Sprintf("Prompt given to the learner: %s\n\nLearner's response: %s", prompt, response)},
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return Rubric{}, err
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL, bytes.NewReader(payload))
	if err != nil {
		return Rubric{}, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Authorization", "Bearer "+c.APIKey)

	httpResponse, err := c.HTTP.Do(httpRequest)
	if err != nil {
		return Rubric{}, err
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(httpResponse.Body, 512))
		return Rubric{}, fmt.Errorf("evaluator: %s: %s", httpResponse.Status, bytes.TrimSpace(message))
	}

	var parsed chatResponse
	if err := json.NewDecoder(httpResponse.Body).Decode(&parsed); err != nil {
		return Rubric{}, fmt.Errorf("evaluator: decode envelope: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return Rubric{}, fmt.Errorf("evaluator: no choices returned")
	}

	var rubric Rubric
	if err := json.Unmarshal([]byte(parsed.Choices[0].Message.Content), &rubric); err != nil {
		return Rubric{}, fmt.Errorf("evaluator: decode rubric: %w", err)
	}
	if err := rubric.Validate(); err != nil {
		return Rubric{}, fmt.Errorf("evaluator: invalid rubric: %w", err)
	}
	return rubric, nil
}
