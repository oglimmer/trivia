package db

import (
	"encoding/json"
	"errors"
	"time"
)

type Game struct {
	ID                     string     `json:"id"`
	Code                   string     `json:"code"`
	Name                   string     `json:"name"`
	State                  string     `json:"state"`
	CurrentQuestionID      *string    `json:"currentQuestionId,omitempty"`
	QuestionState          string     `json:"questionState"`
	QuestionStartedAt      *time.Time `json:"questionStartedAt,omitempty"`
	QuestionClosedAt       *time.Time `json:"questionClosedAt,omitempty"`
	QuestionTimeoutSeconds int        `json:"questionTimeoutSeconds"`
	CreatedAt              time.Time  `json:"createdAt"`
}

type User struct {
	ID        string    `json:"id"`
	GameID    string    `json:"gameId"`
	Name      string    `json:"name"`
	PhotoB64  string    `json:"photoB64,omitempty"`
	Token     string    `json:"token,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type Question struct {
	ID         string          `json:"id"`
	GameID     string          `json:"gameId"`
	UserID     string          `json:"userId"`
	Text       string          `json:"text"`
	PhotoB64   string          `json:"photoB64,omitempty"`
	AnswerType string          `json:"answerType"`
	Options    json.RawMessage `json:"options"`
	Correct    json.RawMessage `json:"correct,omitempty"`
	SortOrder  int             `json:"sortOrder"`
	CreatedAt  time.Time       `json:"createdAt"`
}

type Answer struct {
	ID         string          `json:"id"`
	QuestionID string          `json:"questionId"`
	UserID     string          `json:"userId"`
	Answer     json.RawMessage `json:"answer"`
	ResponseMs int             `json:"responseMs"`
	IsCorrect  bool            `json:"isCorrect"`
	Points     int             `json:"points"`
	CreatedAt  time.Time       `json:"createdAt"`
}

type Score struct {
	UserID   string `json:"userId"`
	UserName string `json:"userName"`
	PhotoB64 string `json:"photoB64,omitempty"`
	Points   int    `json:"points"`
	Correct  int    `json:"correct"`
}

var ErrNotFound = errors.New("not found")
