package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSplitDataURI(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		wantMT   string
		wantData string
	}{
		{
			name:     "png data uri",
			in:       "data:image/png;base64,AAAA",
			wantMT:   "image/png",
			wantData: "AAAA",
		},
		{
			name:     "jpeg data uri",
			in:       "data:image/jpeg;base64,QkJC",
			wantMT:   "image/jpeg",
			wantData: "QkJC",
		},
		{
			name:     "plain base64 defaults to jpeg",
			in:       "AAAA",
			wantMT:   "image/jpeg",
			wantData: "AAAA",
		},
		{
			name:     "malformed data uri falls through",
			in:       "data:image/png-no-comma",
			wantMT:   "image/jpeg",
			wantData: "data:image/png-no-comma",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mt, data := splitDataURI(c.in)
			if mt != c.wantMT || data != c.wantData {
				t.Errorf("splitDataURI(%q) = (%q, %q); want (%q, %q)",
					c.in, mt, data, c.wantMT, c.wantData)
			}
		})
	}
}

func TestSuggestRequiresAPIKey(t *testing.T) {
	c := &Client{} // no APIKey
	_, err := c.Suggest(context.Background(), SuggestRequest{AnswerType: "yesno"})
	if err == nil {
		t.Fatal("expected error when ANTHROPIC_API_KEY missing")
	}
	if !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Fatalf("expected API key error, got: %v", err)
	}
}

func TestSuggestRejectsBadAnswerType(t *testing.T) {
	c := &Client{APIKey: "k"} // key present, type bogus
	_, err := c.Suggest(context.Background(), SuggestRequest{AnswerType: "essay"})
	if err == nil {
		t.Fatal("expected error for unknown answerType")
	}
	if !strings.Contains(err.Error(), "answerType") {
		t.Fatalf("expected answerType error, got: %v", err)
	}
}

// fakeAnthropic spins up an httptest server that pretends to be the Messages
// API. respond returns the JSON body the server should echo back.
func fakeAnthropic(t *testing.T, status int, respond string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.Header.Get("x-api-key") == "" {
			t.Errorf("missing x-api-key header")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Errorf("missing anthropic-version header")
		}
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, respond)
	}))
}

func TestSuggestHappyPath(t *testing.T) {
	innerJSON := `{"text":"What year?","options":["1969","1970","1971","1972"],"correct":1}`
	resp, _ := json.Marshal(map[string]any{
		"content": []map[string]any{{"type": "text", "text": innerJSON}},
	})
	srv := fakeAnthropic(t, 200, string(resp))
	defer srv.Close()

	c := &Client{APIKey: "k", Model: "test", BaseURL: srv.URL, HTTP: srv.Client()}
	got, err := c.Suggest(context.Background(), SuggestRequest{AnswerType: "choice", Hint: "moon"})
	if err != nil {
		t.Fatalf("suggest: %v", err)
	}
	if got.Text != "What year?" {
		t.Errorf("text: got %q", got.Text)
	}
	if len(got.Options) != 4 {
		t.Errorf("options: want 4, got %d", len(got.Options))
	}
	// Numeric JSON deserialised into `any` becomes float64.
	if n, ok := got.Correct.(float64); !ok || n != 1 {
		t.Errorf("correct: want 1, got %v (%T)", got.Correct, got.Correct)
	}
}

func TestSuggestExtractsJSONFromWrappedOutput(t *testing.T) {
	// The model sometimes prefixes prose or wraps the JSON in code fences.
	// The client extracts the first {...} block.
	wrapped := "Sure! Here you go:\n```json\n{\"text\":\"q?\",\"options\":[],\"correct\":42}\n```"
	resp, _ := json.Marshal(map[string]any{
		"content": []map[string]any{{"type": "text", "text": wrapped}},
	})
	srv := fakeAnthropic(t, 200, string(resp))
	defer srv.Close()

	c := &Client{APIKey: "k", Model: "test", BaseURL: srv.URL, HTTP: srv.Client()}
	got, err := c.Suggest(context.Background(), SuggestRequest{AnswerType: "number"})
	if err != nil {
		t.Fatalf("suggest: %v", err)
	}
	if got.Text != "q?" {
		t.Errorf("expected text from inner JSON, got %q", got.Text)
	}
}

func TestSuggestUpstream500(t *testing.T) {
	srv := fakeAnthropic(t, 500, `{"error":{"message":"boom"}}`)
	defer srv.Close()

	c := &Client{APIKey: "k", Model: "test", BaseURL: srv.URL, HTTP: srv.Client()}
	if _, err := c.Suggest(context.Background(), SuggestRequest{AnswerType: "yesno"}); err == nil {
		t.Fatal("expected error from 500")
	}
}

func TestSuggestUpstreamGarbage(t *testing.T) {
	srv := fakeAnthropic(t, 200, `{"content":[{"type":"text","text":"no JSON here at all"}]}`)
	defer srv.Close()

	c := &Client{APIKey: "k", Model: "test", BaseURL: srv.URL, HTTP: srv.Client()}
	_, err := c.Suggest(context.Background(), SuggestRequest{AnswerType: "yesno"})
	if err == nil {
		t.Fatal("expected error when model output has no JSON block")
	}
}
