package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/oglimmer/trivia/backend/internal/ai"
)

func (s *Server) getGameForJoin(w http.ResponseWriter, r *http.Request) {
	g := s.loadGameByCode(w, r)
	if g == nil {
		return
	}
	writeJSON(w, 200, map[string]any{
		"code":        g.Code,
		"name":        g.Name,
		"state":       g.State,
		"scheduledAt": g.ScheduledAt,
	})
}

func (s *Server) joinGame(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Name     string `json:"name"`
		PhotoB64 string `json:"photoB64"`
		Email    string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeErr(w, http.StatusBadRequest, "bad body")
		return
	}
	b.Name = strings.TrimSpace(b.Name)
	b.Email = strings.TrimSpace(b.Email)
	if b.Name == "" {
		writeErr(w, http.StatusBadRequest, "name required")
		return
	}
	g := s.loadGameByCode(w, r)
	if g == nil {
		return
	}
	if g.State != "setup" {
		writeErr(w, http.StatusBadRequest, "game not in setup")
		return
	}
	u, err := s.DB.CreateUser(r.Context(), g.ID, b.Name, b.PhotoB64, b.Email, randomToken(16))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if b.Email != "" {
		s.sendLoginLink(u.Email, u.Name, g.Name, g.Code, u.Token)
	}
	s.broadcastUsers(r.Context(), g.ID)
	writeJSON(w, 200, map[string]any{
		"token":  u.Token,
		"userId": u.ID,
		"gameId": g.ID,
		"code":   g.Code,
	})
}

// sendLoginLink fires off the magic-link email in the background so the
// response doesn't wait on the SMTP server. Errors are logged but never
// surfaced to the caller — a flaky mail server shouldn't block joining.
func (s *Server) sendLoginLink(email, playerName, gameName, gameCode, token string) {
	if s.Mail == nil || email == "" {
		return
	}
	go func() {
		if err := s.Mail.SendLoginLink(email, playerName, gameName, gameCode, token); err != nil {
			log.Printf("send login link to %q: %v", email, err)
		}
	}()
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	u, err := s.playerFromHeader(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, err.Error())
		return
	}
	g, _ := s.DB.GameByID(r.Context(), u.GameID)
	writeJSON(w, 200, map[string]any{
		"user": u,
		"game": g,
	})
}

func (s *Server) updateMe(w http.ResponseWriter, r *http.Request) {
	u, err := s.playerFromHeader(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, err.Error())
		return
	}
	var b struct {
		Name     string `json:"name"`
		PhotoB64 string `json:"photoB64"`
		Email    string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeErr(w, http.StatusBadRequest, "bad body")
		return
	}
	b.Name = strings.TrimSpace(b.Name)
	b.Email = strings.TrimSpace(b.Email)
	if b.Name == "" {
		writeErr(w, http.StatusBadRequest, "name required")
		return
	}
	prevEmail := u.Email
	if err := s.DB.UpdateUser(r.Context(), u.ID, b.Name, b.PhotoB64, b.Email); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Send the magic link only when the email is freshly set or changed —
	// no-op renames shouldn't spam the mailbox.
	if b.Email != "" && b.Email != prevEmail {
		g, gerr := s.DB.GameByID(r.Context(), u.GameID)
		gameName, gameCode := "", ""
		if gerr == nil && g != nil {
			gameName, gameCode = g.Name, g.Code
		}
		tok, terr := s.DB.UserTokenByID(r.Context(), u.ID)
		if terr == nil {
			s.sendLoginLink(b.Email, b.Name, gameName, gameCode, tok)
		}
	}
	s.broadcastUsers(r.Context(), u.GameID)
	w.WriteHeader(204)
}

func (s *Server) listUsersPublic(w http.ResponseWriter, r *http.Request) {
	g := s.loadGameByCode(w, r)
	if g == nil {
		return
	}
	users, err := s.DB.ListUsers(r.Context(), g.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 200, users)
}

func (s *Server) listQuestionsPublic(w http.ResponseWriter, r *http.Request) {
	g := s.loadGameByCode(w, r)
	if g == nil {
		return
	}
	// Only return correct answers if game is finished. Otherwise strip.
	includeCorrect := g.State == "finished"
	qs, err := s.DB.ListQuestions(r.Context(), g.ID, includeCorrect)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 200, qs)
}

type putQuestionBody struct {
	Text       string          `json:"text"`
	PhotoB64   string          `json:"photoB64"`
	AnswerType string          `json:"answerType"`
	Options    json.RawMessage `json:"options"`
	Correct    json.RawMessage `json:"correct"`
}

func (s *Server) putQuestion(w http.ResponseWriter, r *http.Request) {
	u, err := s.playerFromHeader(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, err.Error())
		return
	}
	g := s.loadGameByCode(w, r)
	if g == nil {
		return
	}
	if g.ID != u.GameID {
		writeErr(w, http.StatusForbidden, "wrong game")
		return
	}
	if g.State != "setup" {
		writeErr(w, http.StatusBadRequest, "not in setup")
		return
	}
	var b putQuestionBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeErr(w, http.StatusBadRequest, "bad body")
		return
	}
	if err := validateQuestion(b); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(b.Options) == 0 {
		b.Options = json.RawMessage("[]")
	}
	q, err := s.DB.UpsertQuestion(r.Context(), g.ID, u.ID, b.Text, b.PhotoB64, b.AnswerType, b.Options, b.Correct)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.broadcastQuestionsAdmin(r.Context(), g.ID)
	writeJSON(w, 200, q)
}

func validateQuestion(b putQuestionBody) error {
	if strings.TrimSpace(b.Text) == "" {
		return errors.New("text required")
	}
	if b.PhotoB64 == "" {
		return errors.New("photo required")
	}
	switch b.AnswerType {
	case "yesno":
		var v string
		if err := json.Unmarshal(b.Correct, &v); err != nil {
			return errors.New("correct must be 'yes' or 'no'")
		}
		if v != "yes" && v != "no" {
			return errors.New("correct must be 'yes' or 'no'")
		}
	case "choice":
		var opts []string
		if err := json.Unmarshal(b.Options, &opts); err != nil || len(opts) < 2 || len(opts) > 4 {
			return errors.New("options must be 2-4 strings")
		}
		var idx int
		if err := json.Unmarshal(b.Correct, &idx); err != nil || idx < 0 || idx >= len(opts) {
			return errors.New("correct must be a valid option index")
		}
	case "number":
		var n float64
		if err := json.Unmarshal(b.Correct, &n); err != nil {
			return errors.New("correct must be a number")
		}
	default:
		return errors.New("answerType must be yesno|choice|number")
	}
	return nil
}

func (s *Server) leaderboard(w http.ResponseWriter, r *http.Request) {
	g := s.loadGameByCode(w, r)
	if g == nil {
		return
	}
	sc, err := s.DB.Leaderboard(r.Context(), g.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 200, sc)
}

// ---------- AI ----------

func (s *Server) aiSuggest(w http.ResponseWriter, r *http.Request) {
	var req ai.SuggestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad body")
		return
	}
	res, err := s.AI.Suggest(r.Context(), req)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, 200, res)
}
