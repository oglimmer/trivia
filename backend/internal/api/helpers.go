package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/oglimmer/trivia/backend/internal/db"
)

const (
	defaultQuestionTimeoutSeconds = 30
	minQuestionTimeoutSeconds     = 5
	maxQuestionTimeoutSeconds     = 600
)

// staleUserThreshold is how long a player can be silent before the
// setup→game transition removes them from the lobby.
const staleUserThreshold = 30 * time.Minute

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// serverError logs the underlying error against the request and returns a 500
// to the client. Use this instead of writeErr(w, 500, err.Error()) so that
// server-side problems always leave a trace in the logs, not just in the
// client's response body.
func serverError(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
	writeErr(w, http.StatusInternalServerError, err.Error())
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

// playerFromHeader looks up a player by their X-Player-Token header. It also
// bumps the user's last_seen timestamp so the setup→game cleanup knows the
// player is still around.
func (s *Server) playerFromHeader(r *http.Request) (*db.User, error) {
	tok := r.Header.Get("X-Player-Token")
	if tok == "" {
		return nil, errors.New("missing player token")
	}
	u, err := s.DB.UserByToken(r.Context(), tok)
	if err != nil {
		return nil, err
	}
	if err := s.DB.TouchUserLastSeen(r.Context(), u.ID); err != nil {
		log.Printf("touch last_seen for %s: %v", u.ID, err)
	}
	return u, nil
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
			serverError(w, r, err)
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
