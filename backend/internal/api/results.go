package api

import (
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/oglimmer/trivia/backend/internal/db"
)

// resultsBucket is a single row in a question's answer distribution.
// Value carries the raw answer payload so the frontend can match a player's
// own answer against a bucket without re-parsing the question type.
type resultsBucket struct {
	Label     string          `json:"label"`
	Value     json.RawMessage `json:"value"`
	Count     int             `json:"count"`
	IsCorrect bool            `json:"isCorrect"`
}

type questionResults struct {
	QuestionID     string          `json:"questionId"`
	Text           string          `json:"text"`
	PhotoImageID   *string         `json:"photoImageId,omitempty"`
	AuthorName     string          `json:"authorName,omitempty"`
	AnswerType     string          `json:"answerType"`
	Options        json.RawMessage `json:"options"`
	Correct        json.RawMessage `json:"correct"`
	TotalPlayers   int             `json:"totalPlayers"`
	AnsweredCount  int             `json:"answeredCount"`
	CorrectCount   int             `json:"correctCount"`
	IncorrectCount int             `json:"incorrectCount"`
	NoAnswerCount  int             `json:"noAnswerCount"`
	Distribution   []resultsBucket `json:"distribution"`
}

// results returns per-question answer distributions for the final standings
// page. Only available once the game is finished — same access pattern as
// the public questions list (which already exposes the correct answer at
// that point).
func (s *Server) results(w http.ResponseWriter, r *http.Request) {
	g := s.loadGameByCode(w, r)
	if g == nil {
		return
	}
	if g.State != "finished" {
		writeJSON(w, 200, []questionResults{})
		return
	}
	qs, err := s.DB.ListQuestions(r.Context(), g.ID, true)
	if err != nil {
		serverError(w, r, err)
		return
	}
	users, err := s.DB.ListUsers(r.Context(), g.ID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	authorName := map[string]string{}
	for i := range users {
		authorName[users[i].ID] = users[i].Name
	}
	totalPlayers := len(users)

	out := make([]questionResults, 0, len(qs))
	for _, q := range qs {
		ans, err := s.DB.AnswersForQuestion(r.Context(), q.ID)
		if err != nil {
			serverError(w, r, err)
			return
		}
		out = append(out, buildQuestionResults(q, ans, totalPlayers, authorName[q.UserID]))
	}
	writeJSON(w, 200, out)
}

func buildQuestionResults(q db.Question, ans []db.Answer, totalPlayers int, author string) questionResults {
	res := questionResults{
		QuestionID:    q.ID,
		Text:          q.Text,
		PhotoImageID:  q.PhotoImageID,
		AuthorName:    author,
		AnswerType:    q.AnswerType,
		Options:       q.Options,
		Correct:       q.Correct,
		TotalPlayers:  totalPlayers,
		AnsweredCount: len(ans),
	}
	for _, a := range ans {
		if a.IsCorrect {
			res.CorrectCount++
		} else {
			res.IncorrectCount++
		}
	}
	if n := totalPlayers - res.AnsweredCount; n > 0 {
		res.NoAnswerCount = n
	}
	res.Distribution = distributionFor(q, ans)
	return res
}

func distributionFor(q db.Question, ans []db.Answer) []resultsBucket {
	switch q.AnswerType {
	case "yesno":
		var correct string
		_ = json.Unmarshal(q.Correct, &correct)
		correct = normalizeYesNo(correct)
		var yes, no int
		for _, a := range ans {
			var v string
			if err := json.Unmarshal(a.Answer, &v); err != nil {
				continue
			}
			switch normalizeYesNo(v) {
			case "yes":
				yes++
			case "no":
				no++
			}
		}
		return []resultsBucket{
			{Label: "Yes", Value: json.RawMessage(`"yes"`), Count: yes, IsCorrect: correct == "yes"},
			{Label: "No", Value: json.RawMessage(`"no"`), Count: no, IsCorrect: correct == "no"},
		}
	case "choice":
		var opts []string
		_ = json.Unmarshal(q.Options, &opts)
		var correctIdx int
		_ = json.Unmarshal(q.Correct, &correctIdx)
		counts := make([]int, len(opts))
		for _, a := range ans {
			var idx int
			if err := json.Unmarshal(a.Answer, &idx); err != nil {
				continue
			}
			if idx < 0 || idx >= len(opts) {
				continue
			}
			counts[idx]++
		}
		out := make([]resultsBucket, len(opts))
		for i, label := range opts {
			raw, _ := json.Marshal(i)
			out[i] = resultsBucket{
				Label:     label,
				Value:     raw,
				Count:     counts[i],
				IsCorrect: i == correctIdx,
			}
		}
		return out
	case "number":
		var correct float64
		_ = json.Unmarshal(q.Correct, &correct)
		tol := math.Max(math.Abs(correct)*0.005, 1)
		// Group guesses by their parsed numeric value so identical guesses
		// from multiple players collapse into one bucket.
		counts := map[float64]int{}
		raw := map[float64]json.RawMessage{}
		for _, a := range ans {
			var v float64
			if err := json.Unmarshal(a.Answer, &v); err != nil {
				continue
			}
			counts[v]++
			raw[v] = a.Answer
		}
		out := make([]resultsBucket, 0, len(counts))
		for v, c := range counts {
			out = append(out, resultsBucket{
				Label:     strconv.FormatFloat(v, 'f', -1, 64),
				Value:     raw[v],
				Count:     c,
				IsCorrect: math.Abs(correct-v) <= tol,
			})
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].Count != out[j].Count {
				return out[i].Count > out[j].Count
			}
			return out[i].Label < out[j].Label
		})
		return out
	}
	return []resultsBucket{}
}

// normalizeYesNo mirrors game.normYesNo but is kept local so the API package
// doesn't have to import the scoring package just for a 5-line helper.
func normalizeYesNo(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "yes", "y", "true":
		return "yes"
	case "no", "n", "false":
		return "no"
	}
	return ""
}
