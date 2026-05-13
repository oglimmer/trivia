package api

import (
	"encoding/json"
	"errors"
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
		"code":  g.Code,
		"name":  g.Name,
		"state": g.State,
	})
}

func (s *Server) joinGame(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Name     string `json:"name"`
		PhotoB64 string `json:"photoB64"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeErr(w, http.StatusBadRequest, "bad body")
		return
	}
	b.Name = strings.TrimSpace(b.Name)
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
	u, err := s.DB.CreateUser(r.Context(), g.ID, b.Name, b.PhotoB64, randomToken(16))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.broadcastUsers(r.Context(), g.ID)
	writeJSON(w, 200, map[string]any{
		"token":  u.Token,
		"userId": u.ID,
		"gameId": g.ID,
		"code":   g.Code,
	})
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
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeErr(w, http.StatusBadRequest, "bad body")
		return
	}
	if strings.TrimSpace(b.Name) == "" {
		writeErr(w, http.StatusBadRequest, "name required")
		return
	}
	if err := s.DB.UpdateUser(r.Context(), u.ID, b.Name, b.PhotoB64); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
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
