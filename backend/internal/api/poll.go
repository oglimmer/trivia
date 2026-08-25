package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"

	"github.com/oglimmer/trivia/backend/internal/db"
	"github.com/oglimmer/trivia/backend/internal/game"
)

// pollOptionCount is the fixed number of survey answers behind a poll
// question — the top 5 replies from the pre-event survey.
const pollOptionCount = 5

const maxImportQuestions = 50

// importedAnswer is one survey answer with the number of people who gave it.
type importedAnswer struct {
	Text   string `json:"text"`
	Points int    `json:"points"`
}

type importedQuestion struct {
	Text    string           `json:"text"`
	Answers []importedAnswer `json:"answers"`
}

type importQuestionsBody struct {
	Questions []importedQuestion `json:"questions"`
}

// toHostQuestions validates a pasted question set and converts it into rows.
//
// The options of each question are shuffled on the way in. Without that the
// highest-scoring answer would always sit in the first slot and the whole game
// would collapse into "always tap the top row".
func (b importQuestionsBody) toHostQuestions(shuffle func(n int, swap func(i, j int))) ([]db.HostQuestion, error) {
	if len(b.Questions) == 0 {
		return nil, errors.New("no questions in payload")
	}
	if len(b.Questions) > maxImportQuestions {
		return nil, fmt.Errorf("too many questions (max %d)", maxImportQuestions)
	}
	out := make([]db.HostQuestion, 0, len(b.Questions))
	for i, q := range b.Questions {
		hq, err := buildPollQuestion(q, shuffle)
		if err != nil {
			return nil, fmt.Errorf("question %d: %w", i+1, err)
		}
		out = append(out, hq)
	}
	return out, nil
}

// buildPollQuestion validates one question and turns it into a storable row.
// Both the single-question editor and the bulk import go through here, so the
// rules can never drift between the two.
func buildPollQuestion(q importedQuestion, shuffle func(n int, swap func(i, j int))) (db.HostQuestion, error) {
	text := strings.TrimSpace(q.Text)
	if text == "" {
		return db.HostQuestion{}, errors.New("text is required")
	}
	if len(q.Answers) != pollOptionCount {
		return db.HostQuestion{}, fmt.Errorf("needs exactly %d answers, got %d", pollOptionCount, len(q.Answers))
	}
	opts := make([]game.PollOption, 0, pollOptionCount)
	seen := map[string]bool{}
	for j, a := range q.Answers {
		at := strings.TrimSpace(a.Text)
		if at == "" {
			return db.HostQuestion{}, fmt.Errorf("answer %d: text is required", j+1)
		}
		if seen[strings.ToLower(at)] {
			return db.HostQuestion{}, fmt.Errorf("duplicate answer %q", at)
		}
		seen[strings.ToLower(at)] = true
		if a.Points < 0 {
			return db.HostQuestion{}, fmt.Errorf("answer %d: points must not be negative", j+1)
		}
		opts = append(opts, game.PollOption{Text: at, Points: a.Points})
	}
	// Shuffled on the way in, so the top answer never sits in a predictable
	// slot. The editor always redisplays them ranked by points, so the host
	// never sees — or needs to care about — the stored order.
	shuffle(len(opts), func(x, y int) { opts[x], opts[y] = opts[y], opts[x] })
	raw, err := json.Marshal(opts)
	if err != nil {
		return db.HostQuestion{}, err
	}
	return db.HostQuestion{
		Text:       text,
		AnswerType: "poll",
		Options:    raw,
		// 'correct' is NOT NULL but meaningless for a poll: no single answer
		// is right. The JSON literal null satisfies the column.
		Correct: json.RawMessage("null"),
	}, nil
}

// stripPollPoints removes the point values from a poll question's options,
// leaving only the answer text. The points ARE the answer in this format, so
// shipping them to a phone before the reveal hands every team a perfect score.
//
// Returns the options unchanged for every other question type.
func stripPollPoints(answerType string, options json.RawMessage) json.RawMessage {
	if answerType != "poll" {
		return options
	}
	opts := game.ParsePollOptions(options)
	if opts == nil {
		return json.RawMessage("[]")
	}
	blind := make([]map[string]string, len(opts))
	for i, o := range opts {
		blind[i] = map[string]string{"text": o.Text}
	}
	raw, err := json.Marshal(blind)
	if err != nil {
		return json.RawMessage("[]")
	}
	return raw
}

// sanitizeQuestionsForPlayers blanks poll point values on a list of questions
// unless the caller is allowed to see them.
func sanitizeQuestionsForPlayers(qs []db.Question, reveal bool) {
	if reveal {
		return
	}
	for i := range qs {
		qs[i].Options = stripPollPoints(qs[i].AnswerType, qs[i].Options)
	}
}

// shuffleOptions is the production shuffle; tests inject a deterministic one.
func shuffleOptions(n int, swap func(i, j int)) { rand.Shuffle(n, swap) }
