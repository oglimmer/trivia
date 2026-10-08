package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

const header = "Question ID,Question,Option 1,Points 1,Option 2,Points 2,Option 3,Points 3,Option 4,Points 4,Option 5,Points 5\n"

func TestConvertProducesImportBody(t *testing.T) {
	in := header +
		`1,"Name a souvenir, any souvenir.",Fridge magnet,40,"""I ♥ [City]"" T-shirt",30,Keychain,15,Flag underwear,8,Shot glass,7` + "\n" +
		",,,,,,,,,,,\n" // trailing blank row from a spreadsheet export

	body, err := convert(strings.NewReader(in))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if len(body.Questions) != 1 {
		t.Fatalf("got %d questions, want 1", len(body.Questions))
	}
	q := body.Questions[0]
	if q.Text != "Name a souvenir, any souvenir." {
		t.Errorf("text = %q", q.Text)
	}
	if len(q.Answers) != answersPerQuestion {
		t.Fatalf("got %d answers", len(q.Answers))
	}
	if want := (answer{`"I ♥ [City]" T-shirt`, 30}); q.Answers[1] != want {
		t.Errorf("answer 2 = %+v, want %+v", q.Answers[1], want)
	}
}

func TestConvertOutputMatchesServerShape(t *testing.T) {
	in := header + "1,Q?,A,5,B,4,C,3,D,2,E,1\n"
	body, err := convert(strings.NewReader(in))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	var buf bytes.Buffer
	if err := writeJSON(&buf, body); err != nil {
		t.Fatal(err)
	}
	var got map[string][]map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	q := got["questions"][0]
	if q["text"] != "Q?" {
		t.Errorf("text = %v", q["text"])
	}
	first := q["answers"].([]any)[0].(map[string]any)
	if first["text"] != "A" || first["points"] != float64(5) {
		t.Errorf("first answer = %v", first)
	}
}

func TestConvertHandlesBOMAndColumnOrder(t *testing.T) {
	in := "\ufeffPoints 1,Option 1,Question,Option 2,Points 2,Option 3,Points 3,Option 4,Points 4,Option 5,Points 5\n" +
		"9,A,Q?,B,4,C,3,D,2,E,1\n"
	body, err := convert(strings.NewReader(in))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if got := body.Questions[0].Answers[0]; got != (answer{"A", 9}) {
		t.Errorf("answer 1 = %+v", got)
	}
}

func TestConvertReportsEveryProblemWithLineNumbers(t *testing.T) {
	in := header +
		"1,,A,5,B,4,C,3,D,2,E,1\n" + // line 2: no question text
		"2,Q?,A,5,a,4,C,3,D,2,E,1\n" + // line 3: duplicate (case-insensitive)
		"3,Q?,A,x,B,-1,C,3,D,2,,1\n" // line 4: bad points, negative, empty option

	_, err := convert(strings.NewReader(in))
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{
		"line 2: question text is empty",
		`line 3: Option 2 "a" is a duplicate`,
		`line 4: Points 1 "x" is not a whole number`,
		"line 4: Points 2 must not be negative",
		"line 4: Option 5 is empty",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error is missing %q\n got: %v", want, err)
		}
	}
}

func TestConvertRejectsMissingColumns(t *testing.T) {
	_, err := convert(strings.NewReader("Question,Option 1,Points 1\nQ?,A,1\n"))
	if err == nil || !strings.Contains(err.Error(), `"Option 2"`) {
		t.Fatalf("want missing-column error, got %v", err)
	}
}

func TestConvertRejectsEmptyAndTooMany(t *testing.T) {
	if _, err := convert(strings.NewReader("")); err == nil {
		t.Error("empty file: expected an error")
	}
	if _, err := convert(strings.NewReader(header)); err == nil {
		t.Error("header only: expected an error")
	}

	var sb strings.Builder
	sb.WriteString(header)
	for range maxQuestions + 1 {
		sb.WriteString("1,Q?,A,5,B,4,C,3,D,2,E,1\n")
	}
	_, err := convert(strings.NewReader(sb.String()))
	if err == nil || !strings.Contains(err.Error(), "too many questions") {
		t.Errorf("want too-many error, got %v", err)
	}
}
