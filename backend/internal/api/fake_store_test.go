package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/oglimmer/trivia/backend/internal/db"
)

// fakeStore is an in-memory implementation of Store used by handler tests.
// Behavior follows the real Postgres schema closely enough that test outcomes
// reflect production semantics: unique game codes, upsert-on-conflict for
// questions and answers, leaderboard ordering, etc.
type fakeStore struct {
	mu        sync.Mutex
	games     map[string]*db.Game     // keyed by ID
	users     map[string]*db.User     // keyed by ID
	questions map[string]*db.Question // keyed by ID
	answers   map[string]*db.Answer   // keyed by questionID+"|"+userID
	now       func() time.Time
	seq       int
}

// Compile-time check that fakeStore satisfies Store.
var _ Store = (*fakeStore)(nil)

func newFakeStore() *fakeStore {
	return &fakeStore{
		games:     map[string]*db.Game{},
		users:     map[string]*db.User{},
		questions: map[string]*db.Question{},
		answers:   map[string]*db.Answer{},
		now:       time.Now,
	}
}

func (f *fakeStore) nextID(prefix string) string {
	f.seq++
	return fmt.Sprintf("%s-%d", prefix, f.seq)
}

// ---- Games ----

func (f *fakeStore) CreateGame(_ context.Context, code, name string, timeout int, scheduledAt *time.Time) (*db.Game, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, g := range f.games {
		if g.Code == code {
			return nil, errors.New("duplicate code")
		}
	}
	g := &db.Game{
		ID:                     f.nextID("game"),
		Code:                   code,
		Name:                   name,
		State:                  "setup",
		QuestionState:          "idle",
		QuestionTimeoutSeconds: timeout,
		ScheduledAt:            scheduledAt,
		CreatedAt:              f.now(),
	}
	f.games[g.ID] = g
	return cloneGame(g), nil
}

func (f *fakeStore) GameByCode(_ context.Context, code string) (*db.Game, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, g := range f.games {
		if g.Code == code {
			return cloneGame(g), nil
		}
	}
	return nil, db.ErrNotFound
}

func (f *fakeStore) GameByID(_ context.Context, id string) (*db.Game, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	g, ok := f.games[id]
	if !ok {
		return nil, db.ErrNotFound
	}
	return cloneGame(g), nil
}

func (f *fakeStore) ListGames(_ context.Context) ([]db.Game, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]db.Game, 0, len(f.games))
	for _, g := range f.games {
		out = append(out, *cloneGame(g))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (f *fakeStore) SetGameState(_ context.Context, id, state string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	g, ok := f.games[id]
	if !ok {
		return db.ErrNotFound
	}
	g.State = state
	return nil
}

func (f *fakeStore) SetQuestionTimeout(_ context.Context, id string, seconds int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	g, ok := f.games[id]
	if !ok {
		return db.ErrNotFound
	}
	g.QuestionTimeoutSeconds = seconds
	return nil
}

func (f *fakeStore) SetGameScheduledAt(_ context.Context, id string, scheduledAt *time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	g, ok := f.games[id]
	if !ok {
		return db.ErrNotFound
	}
	if scheduledAt == nil {
		g.ScheduledAt = nil
	} else {
		v := *scheduledAt
		g.ScheduledAt = &v
	}
	return nil
}

func (f *fakeStore) ActiveQuestionGameIDs(_ context.Context) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, g := range f.games {
		if g.QuestionState == "active" {
			out = append(out, g.ID)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (f *fakeStore) DeleteGame(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.games, id)
	for uid, u := range f.users {
		if u.GameID == id {
			delete(f.users, uid)
		}
	}
	for qid, q := range f.questions {
		if q.GameID == id {
			delete(f.questions, qid)
		}
	}
	return nil
}

func (f *fakeStore) ActivateQuestion(_ context.Context, gameID, qID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	g, ok := f.games[gameID]
	if !ok {
		return db.ErrNotFound
	}
	now := f.now()
	g.CurrentQuestionID = &qID
	g.QuestionState = "active"
	g.QuestionStartedAt = &now
	g.QuestionClosedAt = nil
	return nil
}

func (f *fakeStore) CloseQuestion(_ context.Context, gameID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	g, ok := f.games[gameID]
	if !ok {
		return db.ErrNotFound
	}
	now := f.now()
	g.QuestionState = "revealed"
	g.QuestionClosedAt = &now
	return nil
}

func (f *fakeStore) ClearCurrentQuestion(_ context.Context, gameID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	g, ok := f.games[gameID]
	if !ok {
		return db.ErrNotFound
	}
	g.CurrentQuestionID = nil
	g.QuestionState = "idle"
	g.QuestionStartedAt = nil
	g.QuestionClosedAt = nil
	return nil
}

// ---- Users ----

func (f *fakeStore) CreateUser(_ context.Context, gameID, name string, photoImageID *string, email, token string) (*db.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	lower := strings.ToLower(name)
	for _, existing := range f.users {
		if existing.GameID == gameID && strings.ToLower(existing.Name) == lower {
			return nil, db.ErrNameTaken
		}
	}
	now := f.now()
	u := &db.User{
		ID:           f.nextID("user"),
		GameID:       gameID,
		Name:         name,
		PhotoImageID: clonePtr(photoImageID),
		Email:        email,
		Token:        token,
		CreatedAt:    now,
		LastSeen:     now,
	}
	f.users[u.ID] = u
	return cloneUser(u), nil
}

func (f *fakeStore) UpdateUser(_ context.Context, id, name string, photoImageID *string, email string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return db.ErrNotFound
	}
	lower := strings.ToLower(name)
	for otherID, other := range f.users {
		if otherID == id {
			continue
		}
		if other.GameID == u.GameID && strings.ToLower(other.Name) == lower {
			return db.ErrNameTaken
		}
	}
	u.Name = name
	u.PhotoImageID = clonePtr(photoImageID)
	u.Email = email
	return nil
}

func (f *fakeStore) UserByToken(_ context.Context, token string) (*db.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.users {
		if u.Token == token && token != "" {
			return cloneUser(u), nil
		}
	}
	return nil, db.ErrNotFound
}

func (f *fakeStore) DeleteUser(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.users, id)
	return nil
}

func (f *fakeStore) TouchUserLastSeen(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return db.ErrNotFound
	}
	u.LastSeen = f.now()
	return nil
}

func (f *fakeStore) DeleteStaleUsers(_ context.Context, gameID string, cutoff time.Time) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var ids []string
	for id, u := range f.users {
		if u.GameID == gameID && u.LastSeen.Before(cutoff) {
			ids = append(ids, id)
			delete(f.users, id)
		}
	}
	return ids, nil
}

func (f *fakeStore) UserByID(_ context.Context, id string) (*db.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return nil, db.ErrNotFound
	}
	// Matches real ListUsers/UserByID: token is stripped.
	out := cloneUser(u)
	out.Token = ""
	return out, nil
}

func (f *fakeStore) UserTokenByID(_ context.Context, id string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return "", db.ErrNotFound
	}
	return u.Token, nil
}

func (f *fakeStore) ListUsers(_ context.Context, gameID string) ([]db.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []db.User{}
	for _, u := range f.users {
		if u.GameID != gameID {
			continue
		}
		c := *cloneUser(u)
		c.Token = "" // ListUsers strips tokens
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (f *fakeStore) ListAllUsers(_ context.Context) ([]db.AllUser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []db.AllUser{}
	for _, u := range f.users {
		g, ok := f.games[u.GameID]
		if !ok {
			continue
		}
		c := *cloneUser(u)
		c.Token = ""
		out = append(out, db.AllUser{User: c, GameCode: g.Code, GameName: g.Name})
	}
	sort.Slice(out, func(i, j int) bool {
		li, lj := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if li != lj {
			return li < lj
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

// ---- Questions ----

func (f *fakeStore) UpsertQuestion(_ context.Context, gameID, userID, text string, photoImageID *string, answerType string, options, correct json.RawMessage) (*db.Question, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, q := range f.questions {
		if q.GameID == gameID && q.UserID == userID {
			q.Text = text
			q.PhotoImageID = clonePtr(photoImageID)
			q.AnswerType = answerType
			q.Options = options
			q.Correct = correct
			return cloneQuestion(q), nil
		}
	}
	q := &db.Question{
		ID:           f.nextID("q"),
		GameID:       gameID,
		UserID:       userID,
		Text:         text,
		PhotoImageID: clonePtr(photoImageID),
		AnswerType:   answerType,
		Options:      options,
		Correct:      correct,
		SortOrder:    len(f.questions),
		CreatedAt:    f.now(),
	}
	f.questions[q.ID] = q
	return cloneQuestion(q), nil
}

func (f *fakeStore) ListQuestions(_ context.Context, gameID string, includeCorrect bool) ([]db.Question, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []db.Question{}
	for _, q := range f.questions {
		if q.GameID != gameID {
			continue
		}
		c := *cloneQuestion(q)
		if !includeCorrect {
			c.Correct = nil
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (f *fakeStore) QuestionByID(_ context.Context, id string) (*db.Question, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	q, ok := f.questions[id]
	if !ok {
		return nil, db.ErrNotFound
	}
	return cloneQuestion(q), nil
}

func (f *fakeStore) DeleteQuestion(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.questions, id)
	return nil
}

// RandomizeQuestionOrder mimics the production randomization deterministically
// by sorting questions alphabetically by ID and assigning sequential
// sort_order. Tests get a known order without losing the "order changed" signal.
func (f *fakeStore) RandomizeQuestionOrder(_ context.Context, gameID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	ids := []string{}
	for id, q := range f.questions {
		if q.GameID == gameID {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for i, id := range ids {
		f.questions[id].SortOrder = i + 1
	}
	return nil
}

// ---- Answers ----

func answerKey(qID, uID string) string { return qID + "|" + uID }

func (f *fakeStore) SaveAnswer(_ context.Context, questionID, userID string, answer json.RawMessage, responseMs int, isCorrect bool, points int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	k := answerKey(questionID, userID)
	if _, exists := f.answers[k]; exists {
		return nil // ON CONFLICT DO NOTHING
	}
	f.answers[k] = &db.Answer{
		ID:         f.nextID("a"),
		QuestionID: questionID,
		UserID:     userID,
		Answer:     answer,
		ResponseMs: responseMs,
		IsCorrect:  isCorrect,
		Points:     points,
		CreatedAt:  f.now(),
	}
	return nil
}

func (f *fakeStore) UpdateAnswerScore(_ context.Context, questionID, userID string, isCorrect bool, points int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.answers[answerKey(questionID, userID)]
	if !ok {
		return nil
	}
	a.IsCorrect = isCorrect
	a.Points = points
	return nil
}

func (f *fakeStore) AnswersForQuestion(_ context.Context, questionID string) ([]db.Answer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []db.Answer
	for _, a := range f.answers {
		if a.QuestionID == questionID {
			out = append(out, *a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ResponseMs < out[j].ResponseMs })
	return out, nil
}

func (f *fakeStore) Leaderboard(_ context.Context, gameID string) ([]db.Score, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	totals := map[string]*db.Score{}
	for _, u := range f.users {
		if u.GameID != gameID {
			continue
		}
		totals[u.ID] = &db.Score{UserID: u.ID, UserName: u.Name, PhotoImageID: clonePtr(u.PhotoImageID)}
	}
	for _, a := range f.answers {
		q, ok := f.questions[a.QuestionID]
		if !ok || q.GameID != gameID {
			continue
		}
		s, ok := totals[a.UserID]
		if !ok {
			continue
		}
		s.Points += a.Points
		if a.IsCorrect {
			s.Correct++
		}
	}
	out := make([]db.Score, 0, len(totals))
	for _, s := range totals {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Points != out[j].Points {
			return out[i].Points > out[j].Points
		}
		return strings.Compare(out[i].UserName, out[j].UserName) < 0
	})
	return out, nil
}

// ---- cloning helpers ----
//
// Returning pointer aliases would let handlers mutate the store; clone instead.

func cloneGame(g *db.Game) *db.Game {
	c := *g
	if g.CurrentQuestionID != nil {
		v := *g.CurrentQuestionID
		c.CurrentQuestionID = &v
	}
	if g.QuestionStartedAt != nil {
		v := *g.QuestionStartedAt
		c.QuestionStartedAt = &v
	}
	if g.QuestionClosedAt != nil {
		v := *g.QuestionClosedAt
		c.QuestionClosedAt = &v
	}
	if g.ScheduledAt != nil {
		v := *g.ScheduledAt
		c.ScheduledAt = &v
	}
	return &c
}

func cloneUser(u *db.User) *db.User {
	c := *u
	c.PhotoImageID = clonePtr(u.PhotoImageID)
	return &c
}

func cloneQuestion(q *db.Question) *db.Question {
	c := *q
	c.PhotoImageID = clonePtr(q.PhotoImageID)
	if q.Options != nil {
		c.Options = append(json.RawMessage(nil), q.Options...)
	}
	if q.Correct != nil {
		c.Correct = append(json.RawMessage(nil), q.Correct...)
	}
	return &c
}

func clonePtr(p *string) *string {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
