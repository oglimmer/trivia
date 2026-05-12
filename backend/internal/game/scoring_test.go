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
	if !(fast > mid && mid > slow) {
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
