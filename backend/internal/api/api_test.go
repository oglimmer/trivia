package api

import (
	"encoding/json"
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
