package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/oglimmer/trivia/backend/internal/db"
)

const (
	defaultQuestionTimeoutSeconds = 30
	minQuestionTimeoutSeconds     = 5
	maxQuestionTimeoutSeconds     = 600
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func randomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// randomCode generates a short alphanumeric game code.
func randomCode() string {
	const alphabet = "abcdefghijkmnpqrstuvwxyz23456789"
	b := make([]byte, 1)
	out := make([]byte, 4)
	for i := range out {
		_, _ = rand.Read(b)
		out[i] = alphabet[int(b[0])%len(alphabet)]
	}
	return string(out)
}

func clampTimeout(v int) int {
	if v <= 0 {
		return defaultQuestionTimeoutSeconds
	}
	if v < minQuestionTimeoutSeconds {
		return minQuestionTimeoutSeconds
	}
	if v > maxQuestionTimeoutSeconds {
		return maxQuestionTimeoutSeconds
	}
	return v
}

// playerFromHeader looks up a player by their X-Player-Token header.
func (s *Server) playerFromHeader(r *http.Request) (*db.User, error) {
	tok := r.Header.Get("X-Player-Token")
	if tok == "" {
		return nil, errors.New("missing player token")
	}
	return s.DB.UserByToken(r.Context(), tok)
}

// loadGameByCode resolves the {code} URL parameter to a game. On miss it
// writes the appropriate error response and returns nil; the caller should
// just `return` when nil.
func (s *Server) loadGameByCode(w http.ResponseWriter, r *http.Request) *db.Game {
	code := chi.URLParam(r, "code")
	g, err := s.DB.GameByCode(r.Context(), code)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "no game")
		} else {
			writeErr(w, http.StatusInternalServerError, err.Error())
		}
		return nil
	}
	return g
}

// pickNext returns the next question after currentID, or nil if at end.
// Caller passes questions ordered by sort_order. If currentID does not match
// any question (e.g. it was deleted), pickNext restarts from the first.
func pickNext(qs []db.Question, currentID *string) *db.Question {
	if currentID == nil {
		if len(qs) == 0 {
			return nil
		}
		q := qs[0]
		return &q
	}
	for i, q := range qs {
		if q.ID == *currentID {
			if i+1 < len(qs) {
				next := qs[i+1]
				return &next
			}
			return nil
		}
	}
	if len(qs) > 0 {
		q := qs[0]
		return &q
	}
	return nil
}
