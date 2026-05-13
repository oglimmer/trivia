package ai

import (
	"context"
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
