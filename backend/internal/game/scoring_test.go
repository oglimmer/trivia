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

func TestNumberExactAndPartial(t *testing.T) {
	ok, pts := JudgeAnswer("number", 0, raw(t, 100.0), raw(t, 100.0), 0)
	if !ok || pts <= 300 {
		t.Fatalf("expected correct with bonus, got ok=%v pts=%d", ok, pts)
	}
	ok2, pts2 := JudgeAnswer("number", 0, raw(t, 100.0), raw(t, 110.0), 0)
	if ok2 {
		t.Fatalf("expected not exact")
	}
	if pts2 == 0 {
		t.Fatalf("expected partial credit for close guess")
	}
	_, miss := JudgeAnswer("number", 0, raw(t, 100.0), raw(t, 1000.0), 0)
	if miss != 0 {
		t.Fatalf("expected 0 for far guess, got %d", miss)
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
