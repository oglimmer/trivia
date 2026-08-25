package game

import (
	"encoding/json"
	"testing"
)

func pollOpts(t *testing.T, pts ...int) json.RawMessage {
	t.Helper()
	opts := make([]PollOption, len(pts))
	for i, p := range pts {
		opts[i] = PollOption{Text: string(rune('A' + i)), Points: p}
	}
	b, err := json.Marshal(opts)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPollScoresTheChosenOptionsValue(t *testing.T) {
	opts := pollOpts(t, 41, 22, 11, 7, 4)
	// Answered at the very end of the window: no time bonus, raw survey value.
	for idx, want := range map[int]int{0: 41, 1: 22, 4: 4} {
		ok, pts := JudgeAnswer("poll", 5, opts, nil, raw(t, idx), 90_000, 90_000)
		if !ok {
			t.Errorf("option %d: expected a scoring answer", idx)
		}
		if pts != want {
			t.Errorf("option %d: got %d points, want %d", idx, pts, want)
		}
	}
}

func TestPollAwardsTimeBonusOverTheGamesWindow(t *testing.T) {
	opts := pollOpts(t, 40, 20, 10, 5, 5)
	const window = 90_000
	_, instant := JudgeAnswer("poll", 5, opts, nil, raw(t, 0), 0, window)
	_, half := JudgeAnswer("poll", 5, opts, nil, raw(t, 0), window/2, window)
	_, late := JudgeAnswer("poll", 5, opts, nil, raw(t, 0), window, window)

	if instant != 60 { // 40 + 40*0.5
		t.Errorf("instant answer: got %d, want 60", instant)
	}
	if half != 50 { // 40 + 40*0.5*0.5
		t.Errorf("half-window answer: got %d, want 50", half)
	}
	if late != 40 {
		t.Errorf("end-of-window answer: got %d, want 40", late)
	}
}

// The bug this guards: before the window was threaded through, the bonus decayed
// over a hardcoded 30s, so on a 90s question everyone who actually discussed the
// answer got a flat score and only the first 30s separated teams.
func TestTimeBonusTracksTheConfiguredWindowNotThe30sDefault(t *testing.T) {
	opts := pollOpts(t, 40, 20, 10, 5, 5)
	_, pts := JudgeAnswer("poll", 5, opts, nil, raw(t, 0), 60_000, 90_000)
	if pts <= 40 {
		t.Fatalf("a 60s answer in a 90s window should still earn a bonus, got %d", pts)
	}
	_, dead := JudgeAnswer("poll", 5, opts, nil, raw(t, 0), 60_000, 30_000)
	if dead != 40 {
		t.Errorf("a 60s answer in a 30s window should earn no bonus, got %d", dead)
	}
}

func TestPollRejectsOutOfRangeAndUnparsableAnswers(t *testing.T) {
	opts := pollOpts(t, 41, 22, 11, 7, 4)
	for _, answer := range []json.RawMessage{raw(t, 5), raw(t, -1), raw(t, "Pizza"), json.RawMessage(`{}`)} {
		ok, pts := JudgeAnswer("poll", 5, opts, nil, answer, 0, 30_000)
		if ok || pts != 0 {
			t.Errorf("answer %s: got (%v, %d), want (false, 0)", answer, ok, pts)
		}
	}
}

// A zero-point option is possible if a survey answer was imported with a count
// of 0. It should score nothing rather than earning a bonus off a zero base.
func TestPollZeroPointOptionScoresNothing(t *testing.T) {
	opts := pollOpts(t, 41, 0, 0, 0, 0)
	ok, pts := JudgeAnswer("poll", 5, opts, nil, raw(t, 1), 0, 30_000)
	if ok || pts != 0 {
		t.Errorf("got (%v, %d), want (false, 0)", ok, pts)
	}
}

func TestOptionCountHandlesPoll(t *testing.T) {
	if n := OptionCount("poll", pollOpts(t, 1, 2, 3, 4, 5)); n != 5 {
		t.Errorf("got %d, want 5", n)
	}
}

func TestParsePollOptionsOnGarbageReturnsNil(t *testing.T) {
	if opts := ParsePollOptions(json.RawMessage(`"nope"`)); opts != nil {
		t.Errorf("got %v, want nil", opts)
	}
}
