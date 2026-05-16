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

func TestSuggestRequiresAPIKey(t *testing.T) {
	c := &Client{} // no APIKey
	_, err := c.Suggest(context.Background(), SuggestRequest{AnswerType: "yesno"}, nil)
	if err == nil {
		t.Fatal("expected error when ANTHROPIC_API_KEY missing")
	}
	if !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Fatalf("expected API key error, got: %v", err)
	}
}

func TestSuggestRejectsBadAnswerType(t *testing.T) {
	c := &Client{APIKey: "k"} // key present, type bogus
	_, err := c.Suggest(context.Background(), SuggestRequest{AnswerType: "essay"}, nil)
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
	got, err := c.Suggest(context.Background(), SuggestRequest{AnswerType: "choice", Hint: "moon"}, nil)
	if err != nil {
		t.Fatalf("suggest: %v", err)
	}
	if got.Text != "What year?" {
		t.Errorf("text: got %q", got.Text)
	}
	if len(got.Options) != 4 {
		t.Errorf("options: want 4, got %d", len(got.Options))
	}
	// Options are shuffled, but the "correct" index must still point at the
	// original right answer ("1970", index 1 in the upstream payload).
	n, ok := got.Correct.(float64)
	if !ok {
		t.Fatalf("correct: want float64, got %v (%T)", got.Correct, got.Correct)
	}
	if idx := int(n); idx < 0 || idx >= len(got.Options) || got.Options[idx] != "1970" {
		t.Errorf("correct index %d should point at \"1970\", options=%v", idx, got.Options)
	}
}

func TestSuggestShufflesChoiceOptions(t *testing.T) {
	// Run several iterations; with 4 options the chance of *every* run
	// preserving the original order is (1/24)^N, vanishingly small.
	innerJSON := `{"text":"q?","options":["A","B","C","D"],"correct":0}`
	resp, _ := json.Marshal(map[string]any{
		"content": []map[string]any{{"type": "text", "text": innerJSON}},
	})
	srv := fakeAnthropic(t, 200, string(resp))
	defer srv.Close()

	c := &Client{APIKey: "k", Model: "test", BaseURL: srv.URL, HTTP: srv.Client()}
	sawShuffle := false
	for i := 0; i < 40; i++ {
		got, err := c.Suggest(context.Background(), SuggestRequest{AnswerType: "choice"}, nil)
		if err != nil {
			t.Fatalf("suggest: %v", err)
		}
		idx := int(got.Correct.(float64))
		if got.Options[idx] != "A" {
			t.Fatalf("correct index %d should still point at \"A\", got %v", idx, got.Options)
		}
		if idx != 0 || got.Options[0] != "A" || got.Options[1] != "B" || got.Options[2] != "C" || got.Options[3] != "D" {
			sawShuffle = true
		}
	}
	if !sawShuffle {
		t.Errorf("options were never shuffled across 40 runs")
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
	got, err := c.Suggest(context.Background(), SuggestRequest{AnswerType: "number"}, nil)
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
	if _, err := c.Suggest(context.Background(), SuggestRequest{AnswerType: "yesno"}, nil); err == nil {
		t.Fatal("expected error from 500")
	}
}

func TestSuggestUpstreamGarbage(t *testing.T) {
	srv := fakeAnthropic(t, 200, `{"content":[{"type":"text","text":"no JSON here at all"}]}`)
	defer srv.Close()

	c := &Client{APIKey: "k", Model: "test", BaseURL: srv.URL, HTTP: srv.Client()}
	_, err := c.Suggest(context.Background(), SuggestRequest{AnswerType: "yesno"}, nil)
	if err == nil {
		t.Fatal("expected error when model output has no JSON block")
	}
}

func TestSuggestRequestEnablesWebSearch(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"content":[{"type":"text","text":"{\"text\":\"q\",\"options\":[],\"correct\":1}"}]}`)
	}))
	defer srv.Close()

	c := &Client{APIKey: "k", Model: "test", BaseURL: srv.URL, HTTP: srv.Client()}
	if _, err := c.Suggest(context.Background(), SuggestRequest{AnswerType: "number"}, nil); err != nil {
		t.Fatalf("suggest: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(capturedBody, &got); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	tools, ok := got["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("expected exactly 1 tool in request (web_search only — Anthropic auto-injects code_execution for dynamic filtering, declaring it ourselves causes a 400), got %v", got["tools"])
	}
	tool, _ := tools[0].(map[string]any)
	if tool["type"] != "web_search_20260209" || tool["name"] != "web_search" {
		t.Errorf("unexpected tool definition: %v", tool)
	}
}

func TestSuggestParsesLastTextBlockAfterToolUse(t *testing.T) {
	// When web search runs, content is a mix of text, server_tool_use, and
	// web_search_tool_result blocks. The JSON answer is in the *last* text
	// block — an earlier text block may just be the model thinking aloud.
	resp := `{"content":[
		{"type":"text","text":"Let me verify that with a quick search."},
		{"type":"server_tool_use","id":"srvtoolu_1","name":"web_search","input":{"query":"bauxite reserves"}},
		{"type":"web_search_tool_result","tool_use_id":"srvtoolu_1","content":[]},
		{"type":"text","text":"{\"text\":\"q?\",\"options\":[\"A\",\"B\",\"C\",\"D\"],\"correct\":2}"}
	]}`
	srv := fakeAnthropic(t, 200, resp)
	defer srv.Close()

	c := &Client{APIKey: "k", Model: "test", BaseURL: srv.URL, HTTP: srv.Client()}
	got, err := c.Suggest(context.Background(), SuggestRequest{AnswerType: "choice"}, nil)
	if err != nil {
		t.Fatalf("suggest: %v", err)
	}
	if got.Text != "q?" {
		t.Errorf("expected final text block, got %q", got.Text)
	}
	idx := int(got.Correct.(float64))
	if got.Options[idx] != "C" {
		t.Errorf("correct should still point at \"C\" after shuffle, got options=%v idx=%d", got.Options, idx)
	}
}

func TestSuggestSendsImageBlock(t *testing.T) {
	// When an Image is provided, the user message must include an image block
	// with base64-encoded data of the right media type.
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"content":[{"type":"text","text":"{\"text\":\"q\",\"options\":[],\"correct\":1}"}]}`)
	}))
	defer srv.Close()

	c := &Client{APIKey: "k", Model: "test", BaseURL: srv.URL, HTTP: srv.Client()}
	img := &Image{MediaType: "image/png", Data: []byte{0x89, 0x50, 0x4E, 0x47}}
	if _, err := c.Suggest(context.Background(), SuggestRequest{AnswerType: "number"}, img); err != nil {
		t.Fatalf("suggest: %v", err)
	}
	if !strings.Contains(string(capturedBody), `"type":"image"`) {
		t.Fatalf("request body missing image block: %s", capturedBody)
	}
	if !strings.Contains(string(capturedBody), `"media_type":"image/png"`) {
		t.Fatalf("request body missing media_type: %s", capturedBody)
	}
	// iVBORw== is the base64 encoding of 0x89 0x50 0x4E 0x47 (PNG magic).
	if !strings.Contains(string(capturedBody), `iVBORw==`) {
		t.Fatalf("request body missing base64-encoded PNG magic: %s", capturedBody)
	}
}
