package game

import (
	"encoding/json"
	"testing"
)

func raw(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestYesNoCorrectAndTimeBonus(t *testing.T) {
	ok, pts := JudgeAnswer("yesno", 2, raw(t, "yes"), raw(t, "yes"), 0)
	if !ok {
		t.Fatalf("expected correct")
	}
	if pts != 150 { // 100 base + 50 bonus at 0ms
		t.Fatalf("expected 150, got %d", pts)
	}
}

func TestYesNoWrong(t *testing.T) {
	ok, pts := JudgeAnswer("yesno", 2, raw(t, "yes"), raw(t, "no"), 1000)
	if ok || pts != 0 {
		t.Fatalf("expected wrong/0, got ok=%v pts=%d", ok, pts)
	}
}

func TestChoiceScalesWithOptionCount(t *testing.T) {
	for _, tc := range []struct {
		opts, want int
	}{{2, 100}, {3, 200}, {4, 300}} {
		_, pts := JudgeAnswer("choice", tc.opts, raw(t, 1), raw(t, 1), AnswerWindowMs)
		if pts != tc.want {
			t.Fatalf("options=%d: expected %d, got %d", tc.opts, tc.want, pts)
		}
	}
}

func TestTimeBonusDecays(t *testing.T) {
	_, fast := JudgeAnswer("choice", 3, raw(t, 0), raw(t, 0), 0)
	_, mid := JudgeAnswer("choice", 3, raw(t, 0), raw(t, 0), 15_000)
	_, slow := JudgeAnswer("choice", 3, raw(t, 0), raw(t, 0), 30_000)
	if fast <= mid || mid <= slow {
		t.Fatalf("expected monotonic decay, got fast=%d mid=%d slow=%d", fast, mid, slow)
	}
	if slow != 200 {
		t.Fatalf("expected base 200 at window end, got %d", slow)
	}
}

func TestJudgeNumberDefersScoring(t *testing.T) {
	// Number answers are scored later via ScoreNumberAnswers; JudgeAnswer
	// returns a 0/false placeholder regardless of correctness or speed.
	ok, pts := JudgeAnswer("number", 0, raw(t, 100.0), raw(t, 100.0), 0)
	if ok || pts != 0 {
		t.Fatalf("expected deferred (false, 0), got ok=%v pts=%d", ok, pts)
	}
}

func TestScoreNumberExactGetsTimeBonus(t *testing.T) {
	scores := ScoreNumberAnswers(raw(t, 100.0), []NumberAnswer{
		{UserID: "a", Answer: raw(t, 100.0), ResponseMs: 0},
	})
	if len(scores) != 1 {
		t.Fatalf("want 1 score, got %d", len(scores))
	}
	if !scores[0].IsCorrect {
		t.Fatalf("expected exact match to be correct")
	}
	if scores[0].Points <= 300 {
		t.Fatalf("expected base + time bonus > 300, got %d", scores[0].Points)
	}
}

func TestScoreNumberTopThreeOnly(t *testing.T) {
	scores := ScoreNumberAnswers(raw(t, 100.0), []NumberAnswer{
		{UserID: "a", Answer: raw(t, 103.0), ResponseMs: 5000},
		{UserID: "b", Answer: raw(t, 107.0), ResponseMs: 5000},
		{UserID: "c", Answer: raw(t, 112.0), ResponseMs: 5000},
		{UserID: "d", Answer: raw(t, 120.0), ResponseMs: 5000},
	})
	byUser := map[string]NumberScore{}
	for _, s := range scores {
		byUser[s.UserID] = s
	}
	if byUser["a"].Points <= byUser["b"].Points {
		t.Fatalf("closer guess should score higher: a=%d b=%d", byUser["a"].Points, byUser["b"].Points)
	}
	if byUser["b"].Points <= byUser["c"].Points {
		t.Fatalf("closer guess should score higher: b=%d c=%d", byUser["b"].Points, byUser["c"].Points)
	}
	if byUser["c"].Points == 0 {
		t.Fatalf("3rd place should score, got 0")
	}
	if byUser["d"].Points != 0 {
		t.Fatalf("4th place should not score, got %d", byUser["d"].Points)
	}
	for _, s := range scores {
		if s.IsCorrect {
			t.Fatalf("no exact matches: %+v should not be correct", s)
		}
	}
}

func TestScoreNumberNoTimeBonusForNonExact(t *testing.T) {
	// Same guess, different response times — non-exact guesses get no time
	// bonus, so points should match.
	fast := ScoreNumberAnswers(raw(t, 100.0), []NumberAnswer{
		{UserID: "a", Answer: raw(t, 110.0), ResponseMs: 0},
	})
	slow := ScoreNumberAnswers(raw(t, 100.0), []NumberAnswer{
		{UserID: "a", Answer: raw(t, 110.0), ResponseMs: 29_000},
	})
	if fast[0].Points != slow[0].Points {
		t.Fatalf("non-exact guesses must not get a time bonus: fast=%d slow=%d", fast[0].Points, slow[0].Points)
	}
}

func TestScoreNumberWildGuessGetsZero(t *testing.T) {
	// A lone guess that's wildly off should not earn points even though it's
	// technically the top-1 — closeness collapses to 0.
	scores := ScoreNumberAnswers(raw(t, 100.0), []NumberAnswer{
		{UserID: "a", Answer: raw(t, 100000.0), ResponseMs: 0},
	})
	if scores[0].Points != 0 {
		t.Fatalf("wild guess should get 0, got %d", scores[0].Points)
	}
}

func TestOptionCount(t *testing.T) {
	if got := OptionCount("choice", raw(t, []string{"a", "b", "c"})); got != 3 {
		t.Fatalf("expected 3, got %d", got)
	}
	if got := OptionCount("yesno", raw(t, nil)); got != 2 {
		t.Fatalf("expected 2 for yesno, got %d", got)
	}
}

func TestOptionCountNumberAndMalformed(t *testing.T) {
	if got := OptionCount("number", raw(t, nil)); got != 0 {
		t.Errorf("expected 0 for number, got %d", got)
	}
	if got := OptionCount("choice", json.RawMessage(`not-json`)); got != 0 {
		t.Errorf("expected 0 for malformed choice options, got %d", got)
	}
}

func TestBasePointsManyOptions(t *testing.T) {
	// Beyond the named 2/3/4 case there is a linear extension at 100 per
	// additional option. Lock that contract in.
	if got := basePoints("choice", 5); got != 400 {
		t.Errorf("5 options: want 400, got %d", got)
	}
	if got := basePoints("choice", 6); got != 500 {
		t.Errorf("6 options: want 500, got %d", got)
	}
	if got := basePoints("choice", 1); got != 100 {
		t.Errorf("1 option (degenerate): want 100, got %d", got)
	}
	if got := basePoints("unknown-type", 0); got != 100 {
		t.Errorf("unknown answerType should fall back to 100, got %d", got)
	}
}

func TestNormYesNo(t *testing.T) {
	cases := []struct{ in, want string }{
		{"yes", "yes"}, {"Yes", "yes"}, {"YES", "yes"},
		{"y", "yes"}, {"Y", "yes"},
		{"true", "yes"}, {"True", "yes"},
		{"no", "no"}, {"No", "no"}, {"NO", "no"},
		{"n", "no"}, {"N", "no"},
		{"false", "no"}, {"False", "no"},
		{"maybe", "maybe"}, // unrecognized passes through
		{"", ""},
	}
	for _, c := range cases {
		if got := normYesNo(c.in); got != c.want {
			t.Errorf("normYesNo(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestJudgeAnswerInvalidJSONReturnsZero(t *testing.T) {
	// Malformed JSON on either side must produce wrong/0, not a panic.
	cases := []struct {
		name       string
		answerType string
		correct    json.RawMessage
		answer     json.RawMessage
	}{
		{"choice malformed correct", "choice", json.RawMessage(`not-json`), raw(t, 0)},
		{"choice malformed answer", "choice", raw(t, 0), json.RawMessage(`not-json`)},
		{"yesno empty correct", "yesno", json.RawMessage(``), raw(t, "yes")},
		{"yesno empty answer", "yesno", raw(t, "yes"), json.RawMessage(``)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ok, pts := JudgeAnswer(c.answerType, 2, c.correct, c.answer, 0)
			if ok || pts != 0 {
				t.Fatalf("expected (false, 0), got (%v, %d)", ok, pts)
			}
		})
	}
}

func TestJudgeAnswerUnknownType(t *testing.T) {
	ok, pts := JudgeAnswer("essay", 0, raw(t, "a"), raw(t, "a"), 0)
	if ok || pts != 0 {
		t.Fatalf("unknown answerType should be (false, 0), got (%v, %d)", ok, pts)
	}
}

func TestJudgeChoiceWrongAnswerIsZero(t *testing.T) {
	ok, pts := JudgeAnswer("choice", 3, raw(t, 0), raw(t, 1), 0)
	if ok || pts != 0 {
		t.Fatalf("wrong choice should be (false, 0), got (%v, %d)", ok, pts)
	}
}

func TestScoreNumberEmpty(t *testing.T) {
	if scores := ScoreNumberAnswers(raw(t, 100.0), nil); len(scores) != 0 {
		t.Fatalf("nil input should yield empty result, got %d entries", len(scores))
	}
	if scores := ScoreNumberAnswers(raw(t, 100.0), []NumberAnswer{}); len(scores) != 0 {
		t.Fatalf("empty input should yield empty result, got %d entries", len(scores))
	}
}

func TestScoreNumberBadCorrectJSON(t *testing.T) {
	// If `correct` cannot be parsed as a number, no one scores — but the
	// returned slice still has one entry per submitted answer (with zero values).
	scores := ScoreNumberAnswers(json.RawMessage(`"oops"`), []NumberAnswer{
		{UserID: "a", Answer: raw(t, 100.0), ResponseMs: 0},
	})
	if len(scores) != 1 {
		t.Fatalf("want 1 score entry, got %d", len(scores))
	}
	if scores[0].Points != 0 || scores[0].IsCorrect {
		t.Fatalf("malformed correct should yield zero score, got %+v", scores[0])
	}
}

func TestScoreNumberSkipsMalformedAnswer(t *testing.T) {
	// A player who submitted unparseable JSON for a number question must not
	// be awarded points, but must still appear in the result slice.
	scores := ScoreNumberAnswers(raw(t, 100.0), []NumberAnswer{
		{UserID: "good", Answer: raw(t, 100.0), ResponseMs: 0},
		{UserID: "bad", Answer: json.RawMessage(`not-a-number`), ResponseMs: 0},
	})
	if len(scores) != 2 {
		t.Fatalf("want 2 score entries, got %d", len(scores))
	}
	byUser := map[string]NumberScore{}
	for _, s := range scores {
		byUser[s.UserID] = s
	}
	if !byUser["good"].IsCorrect || byUser["good"].Points == 0 {
		t.Fatalf("good submitter should score, got %+v", byUser["good"])
	}
	if byUser["bad"].Points != 0 || byUser["bad"].IsCorrect {
		t.Fatalf("malformed answer should score zero, got %+v", byUser["bad"])
	}
}

func TestScoreNumberTieBrokenByResponseTime(t *testing.T) {
	// Two non-exact guesses equidistant from the correct answer — the faster
	// responder must take rank 1 (heavier weight).
	scores := ScoreNumberAnswers(raw(t, 100.0), []NumberAnswer{
		{UserID: "slow", Answer: raw(t, 110.0), ResponseMs: 10_000},
		{UserID: "fast", Answer: raw(t, 110.0), ResponseMs: 1_000},
	})
	byUser := map[string]NumberScore{}
	for _, s := range scores {
		byUser[s.UserID] = s
	}
	if byUser["fast"].Points <= byUser["slow"].Points {
		t.Fatalf("faster responder must outrank slower on tie: fast=%d slow=%d",
			byUser["fast"].Points, byUser["slow"].Points)
	}
}
