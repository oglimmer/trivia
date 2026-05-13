package game

import (
	"encoding/json"
	"math"
	"sort"
)

// AnswerWindowMs is the time after which no time bonus is awarded.
const AnswerWindowMs = 30_000

// basePoints derives a difficulty floor from the answer type and option count.
// Reasoning: more options/harder answer space -> more points.
//
//	yes/no                 100
//	choice with 2 options  100
//	choice with 3 options  200
//	choice with 4 options  300
//	number                 300 (open-ended)
func basePoints(answerType string, optionCount int) int {
	switch answerType {
	case "yesno":
		return 100
	case "choice":
		switch optionCount {
		case 2:
			return 100
		case 3:
			return 200
		case 4:
			return 300
		default:
			if optionCount > 4 {
				return 300 + (optionCount-4)*100
			}
			return 100
		}
	case "number":
		return 300
	}
	return 100
}

// timeBonus returns a value in [0, base/2] that decays linearly with response time.
// A response at 0ms gets the full bonus, at AnswerWindowMs or later gets 0.
func timeBonus(base, responseMs int) int {
	if responseMs < 0 {
		responseMs = 0
	}
	if responseMs >= AnswerWindowMs {
		return 0
	}
	frac := 1.0 - float64(responseMs)/float64(AnswerWindowMs)
	return int(math.Round(float64(base) * 0.5 * frac))
}

// JudgeAnswer determines correctness and awards points for yes/no and choice
// answers. Number answers are scored later via ScoreNumberAnswers (called at
// reveal time) because their points depend on the whole field of guesses.
func JudgeAnswer(answerType string, optionCount int, correct, answer json.RawMessage, responseMs int) (isCorrect bool, points int) {
	base := basePoints(answerType, optionCount)
	switch answerType {
	case "yesno":
		var c, a string
		_ = json.Unmarshal(correct, &c)
		_ = json.Unmarshal(answer, &a)
		if c == "" || a == "" {
			return false, 0
		}
		if normYesNo(c) == normYesNo(a) {
			return true, base + timeBonus(base, responseMs)
		}
		return false, 0
	case "choice":
		var c, a int
		if err := json.Unmarshal(correct, &c); err != nil {
			return false, 0
		}
		if err := json.Unmarshal(answer, &a); err != nil {
			return false, 0
		}
		if c == a {
			return true, base + timeBonus(base, responseMs)
		}
		return false, 0
	case "number":
		// Deferred: ScoreNumberAnswers handles this at reveal time.
		return false, 0
	}
	return false, 0
}

// NumberAnswer is one player's submission for a number question.
type NumberAnswer struct {
	UserID     string
	Answer     json.RawMessage
	ResponseMs int
}

// NumberScore is the awarded score for a single NumberAnswer after ranking.
type NumberScore struct {
	UserID    string
	IsCorrect bool
	Points    int
}

// numberExactTolerance returns the absolute tolerance for treating a guess as
// exact. Mirrors the legacy single-answer rule (0.5% of |c| or 1, whichever larger).
func numberExactTolerance(c float64) float64 {
	return math.Max(math.Abs(c)*0.005, 1)
}

// ScoreNumberAnswers ranks number answers by distance to `correct` and awards
// points to the three closest guesses. Points scale with closeness. A guess
// within the exact tolerance gets the full base plus a time bonus; non-exact
// top-3 guesses get partial credit and no time bonus.
//
// Rank weights of [1.0, 0.66, 0.33] are applied to non-exact top-3 finishers.
// Closeness is `max(0, 1 - diff/scale)` where `scale = max(|c|*0.5, 10)`,
// so very wild guesses earn little even if they happen to make the top 3.
func ScoreNumberAnswers(correct json.RawMessage, answers []NumberAnswer) []NumberScore {
	out := make([]NumberScore, len(answers))
	for i, a := range answers {
		out[i] = NumberScore{UserID: a.UserID}
	}
	if len(answers) == 0 {
		return out
	}
	var c float64
	if err := json.Unmarshal(correct, &c); err != nil {
		return out
	}

	type ranked struct {
		idx        int
		diff       float64
		valid      bool
		exact      bool
		responseMs int
	}
	tol := numberExactTolerance(c)
	rs := make([]ranked, 0, len(answers))
	for i, a := range answers {
		var v float64
		if err := json.Unmarshal(a.Answer, &v); err != nil {
			continue
		}
		d := math.Abs(c - v)
		rs = append(rs, ranked{
			idx:        i,
			diff:       d,
			valid:      true,
			exact:      d <= tol,
			responseMs: a.ResponseMs,
		})
	}
	sort.SliceStable(rs, func(i, j int) bool {
		if rs[i].diff != rs[j].diff {
			return rs[i].diff < rs[j].diff
		}
		return rs[i].responseMs < rs[j].responseMs
	})

	base := basePoints("number", 0)
	rankWeights := [3]float64{1.0, 0.66, 0.33}
	scale := math.Max(math.Abs(c)*0.5, 10)

	for rank, r := range rs {
		if r.exact {
			out[r.idx] = NumberScore{
				UserID:    answers[r.idx].UserID,
				IsCorrect: true,
				Points:    base + timeBonus(base, r.responseMs),
			}
			continue
		}
		if rank >= len(rankWeights) {
			continue
		}
		closeness := math.Max(0, 1.0-r.diff/scale)
		if closeness <= 0 {
			continue
		}
		out[r.idx] = NumberScore{
			UserID: answers[r.idx].UserID,
			Points: int(math.Round(float64(base) * rankWeights[rank] * closeness)),
		}
	}
	return out
}

func normYesNo(s string) string {
	switch s {
	case "yes", "Yes", "YES", "y", "Y", "true", "True":
		return "yes"
	case "no", "No", "NO", "n", "N", "false", "False":
		return "no"
	}
	return s
}

// OptionCount inspects the raw options JSON to extract a count for 'choice'.
func OptionCount(answerType string, options json.RawMessage) int {
	if answerType != "choice" {
		if answerType == "yesno" {
			return 2
		}
		return 0
	}
	var arr []any
	if err := json.Unmarshal(options, &arr); err != nil {
		return 0
	}
	return len(arr)
}
