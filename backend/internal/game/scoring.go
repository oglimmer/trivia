package game

import (
	"encoding/json"
	"math"
)

// AnswerWindowMs is the time after which no time bonus is awarded.
const AnswerWindowMs = 30_000

// basePoints derives a difficulty floor from the answer type and option count.
// Reasoning: more options/harder answer space -> more points.
//   yes/no                 100
//   choice with 2 options  100
//   choice with 3 options  200
//   choice with 4 options  300
//   number                 300 (open-ended)
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

// JudgeAnswer determines correctness and awards points.
// `correct` and `answer` are raw JSON values matching the question's answer_type.
// For number questions, partial credit is awarded based on proximity.
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
		var c, a float64
		if err := json.Unmarshal(correct, &c); err != nil {
			return false, 0
		}
		if err := json.Unmarshal(answer, &a); err != nil {
			return false, 0
		}
		// Exact (within 0.5% or 1 absolute, whichever is larger) -> full credit + bonus.
		// Otherwise scale by closeness: max 1x base, 0 below threshold.
		tol := math.Max(math.Abs(c)*0.005, 1)
		diff := math.Abs(c - a)
		if diff <= tol {
			return true, base + timeBonus(base, responseMs)
		}
		// Partial credit window: if within 25% of |c| (or 10 if c==0), award partial.
		window := math.Max(math.Abs(c)*0.25, 10)
		if diff < window {
			frac := 1.0 - diff/window
			p := int(math.Round(float64(base) * 0.6 * frac))
			return false, p
		}
		return false, 0
	}
	return false, 0
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
