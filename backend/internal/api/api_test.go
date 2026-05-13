package api

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/oglimmer/trivia/backend/internal/db"
)

func TestValidateQuestion(t *testing.T) {
	cases := []struct {
		name string
		body putQuestionBody
		ok   bool
	}{
		{
			name: "valid yesno",
			body: putQuestionBody{Text: "real?", PhotoB64: "x", AnswerType: "yesno", Correct: json.RawMessage(`"yes"`)},
			ok:   true,
		},
		{
			name: "yesno bad correct",
			body: putQuestionBody{Text: "real?", PhotoB64: "x", AnswerType: "yesno", Correct: json.RawMessage(`"maybe"`)},
			ok:   false,
		},
		{
			name: "valid choice",
			body: putQuestionBody{Text: "?", PhotoB64: "x", AnswerType: "choice",
				Options: json.RawMessage(`["a","b","c"]`), Correct: json.RawMessage(`1`)},
			ok: true,
		},
		{
			name: "choice index out of range",
			body: putQuestionBody{Text: "?", PhotoB64: "x", AnswerType: "choice",
				Options: json.RawMessage(`["a","b"]`), Correct: json.RawMessage(`5`)},
			ok: false,
		},
		{
			name: "choice too many options",
			body: putQuestionBody{Text: "?", PhotoB64: "x", AnswerType: "choice",
				Options: json.RawMessage(`["a","b","c","d","e"]`), Correct: json.RawMessage(`0`)},
			ok: false,
		},
		{
			name: "valid number",
			body: putQuestionBody{Text: "?", PhotoB64: "x", AnswerType: "number", Correct: json.RawMessage(`42`)},
			ok:   true,
		},
		{
			name: "missing photo",
			body: putQuestionBody{Text: "?", AnswerType: "yesno", Correct: json.RawMessage(`"yes"`)},
			ok:   false,
		},
		{
			name: "unknown type",
			body: putQuestionBody{Text: "?", PhotoB64: "x", AnswerType: "essay", Correct: json.RawMessage(`"foo"`)},
			ok:   false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateQuestion(c.body)
			if c.ok && err != nil {
				t.Fatalf("expected ok, got %v", err)
			}
			if !c.ok && err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}

func TestPickNext(t *testing.T) {
	qs := []db.Question{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	if n := pickNext(qs, nil); n == nil || n.ID != "a" {
		t.Fatalf("expected a, got %+v", n)
	}
	b := "b"
	if n := pickNext(qs, &b); n == nil || n.ID != "c" {
		t.Fatalf("expected c, got %+v", n)
	}
	c := "c"
	if n := pickNext(qs, &c); n != nil {
		t.Fatalf("expected nil at end, got %+v", n)
	}
	if n := pickNext(nil, nil); n != nil {
		t.Fatalf("expected nil for empty list")
	}
}

func TestRandomCodeShape(t *testing.T) {
	for i := 0; i < 5; i++ {
		c := randomCode()
		if len(c) != 4 {
			t.Fatalf("expected len 4, got %q", c)
		}
	}
}

func TestRandomCodeAlphabet(t *testing.T) {
	// The alphabet deliberately omits ambiguous glyphs (0/o, 1/l). Any leak of
	// those into the generated codes is a regression.
	const allowed = "abcdefghijkmnpqrstuvwxyz23456789"
	for i := 0; i < 200; i++ {
		c := randomCode()
		for _, ch := range c {
			if !strings.ContainsRune(allowed, ch) {
				t.Fatalf("rune %q in %q not in allowed alphabet", ch, c)
			}
		}
	}
}

func TestClampTimeout(t *testing.T) {
	cases := []struct {
		name string
		in   int
		want int
	}{
		{"zero -> default", 0, defaultQuestionTimeoutSeconds},
		{"negative -> default", -5, defaultQuestionTimeoutSeconds},
		{"below min -> min", minQuestionTimeoutSeconds - 1, minQuestionTimeoutSeconds},
		{"at min", minQuestionTimeoutSeconds, minQuestionTimeoutSeconds},
		{"in range passes through", 45, 45},
		{"at max", maxQuestionTimeoutSeconds, maxQuestionTimeoutSeconds},
		{"above max -> max", maxQuestionTimeoutSeconds + 100, maxQuestionTimeoutSeconds},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := clampTimeout(c.in); got != c.want {
				t.Errorf("clampTimeout(%d) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

func TestRandomTokenLength(t *testing.T) {
	for _, n := range []int{8, 16, 32} {
		got := randomToken(n)
		// Hex-encoded, so length is 2*n.
		if len(got) != 2*n {
			t.Errorf("randomToken(%d) len = %d, want %d", n, len(got), 2*n)
		}
		for _, ch := range got {
			if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
				t.Fatalf("randomToken produced non-hex char %q in %q", ch, got)
			}
		}
	}
}

func TestRandomTokensDiffer(t *testing.T) {
	// Sanity: 16-byte tokens shouldn't collide across a handful of calls.
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		tok := randomToken(16)
		if seen[tok] {
			t.Fatalf("collision after %d tokens: %s", i+1, tok)
		}
		seen[tok] = true
	}
}
