package eval

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func envelope(t *testing.T, content string) []byte {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"choices": []map[string]any{
			{"message": map[string]any{"content": content}},
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return payload
}

func TestEvaluateValidatesRubric(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("authorization = %q", got)
		}
		var request map[string]any
		json.NewDecoder(r.Body).Decode(&request)
		if request["temperature"] != float64(0) {
			t.Errorf("temperature = %v, want 0", request["temperature"])
		}
		w.Write(envelope(t, `{"meaning":2,"grammar":1,"naturalness":1,"register":2,"feedback":"Good."}`))
	}))
	defer server.Close()

	client := &Client{APIKey: "test-key", BaseURL: server.URL, Model: DefaultModel, HTTP: server.Client()}
	rubric, err := client.Evaluate(context.Background(), "Say something", "すしをたべたいです")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if rubric.Meaning != 2 || rubric.Feedback != "Good." {
		t.Fatalf("rubric = %+v", rubric)
	}
}

// Every failure mode must be an error, so the caller saves the response ungraded
// instead of inventing a score (D3).
func TestEvaluateFailsClosed(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		key     string
	}{
		{
			name: "missing key",
			key:  "",
			handler: func(w http.ResponseWriter, r *http.Request) {
				t.Error("no request should be made without a key")
			},
		},
		{
			name: "non-200",
			key:  "k",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "rate limited", http.StatusTooManyRequests)
			},
		},
		{
			name: "malformed envelope",
			key:  "k",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("not json"))
			},
		},
		{
			name: "no choices",
			key:  "k",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{"choices":[]}`))
			},
		},
		{
			name: "rubric is not json",
			key:  "k",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write(envelope(t, "Sure! Here is my grade: good."))
			},
		},
		{
			name: "score out of range",
			key:  "k",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write(envelope(t, `{"meaning":5,"grammar":1,"naturalness":1,"register":1,"feedback":"x"}`))
			},
		},
		{
			name: "missing feedback",
			key:  "k",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write(envelope(t, `{"meaning":2,"grammar":2,"naturalness":2,"register":2,"feedback":""}`))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(tc.handler)
			defer server.Close()

			client := &Client{APIKey: tc.key, BaseURL: server.URL, Model: DefaultModel, HTTP: server.Client()}
			if _, err := client.Evaluate(context.Background(), "Say something", "すし"); err == nil {
				t.Fatal("expected an error so the caller saves the response ungraded")
			}
		})
	}
}
