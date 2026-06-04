package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/oglimmer/trivia/backend/internal/db"
	"github.com/oglimmer/trivia/backend/internal/ws"
)

// myVote returns the question the authenticated player has voted for as their
// best question, or an empty string if they have not voted yet. Used by the
// results page to lock the UI on reload.
func (s *Server) myVote(w http.ResponseWriter, r *http.Request) {
	g := s.loadGameByCode(w, r)
	if g == nil {
		return
	}
	u, err := s.playerFromHeader(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	qid, err := s.DB.UserVote(r.Context(), g.ID, u.ID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"questionId": qid})
}

// adminVotes returns the best-question vote tally keyed by question id. Admin
// only — players never see counts, to keep the vote unbiased.
func (s *Server) adminVotes(w http.ResponseWriter, r *http.Request) {
	g := s.loadGameByCode(w, r)
	if g == nil {
		return
	}
	counts, err := s.DB.VoteCounts(r.Context(), g.ID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, 200, counts)
}

type voteBody struct {
	QuestionID string `json:"questionId"`
}

// castVote records a player's single best-question vote. Voting is only allowed
// once the game is finished, and each player may vote exactly once — the vote
// is final and cannot be changed or withdrawn. A repeat call is a no-op that
// returns the player's existing vote.
func (s *Server) castVote(w http.ResponseWriter, r *http.Request) {
	g := s.loadGameByCode(w, r)
	if g == nil {
		return
	}
	if g.State != "finished" {
		writeErr(w, http.StatusConflict, "voting is only open once the game has finished")
		return
	}
	u, err := s.playerFromHeader(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if u.GameID != g.ID {
		writeErr(w, http.StatusForbidden, "not a player in this game")
		return
	}

	var b voteBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.QuestionID == "" {
		writeErr(w, http.StatusBadRequest, "questionId is required")
		return
	}
	q, err := s.DB.QuestionByID(r.Context(), b.QuestionID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "no such question")
		} else {
			serverError(w, r, err)
		}
		return
	}
	if q.GameID != g.ID {
		writeErr(w, http.StatusBadRequest, "question is not part of this game")
		return
	}

	cast, err := s.DB.SaveVote(r.Context(), g.ID, q.ID, u.ID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	// Resolve the player's effective vote: if they had already voted, cast is
	// false and their original choice stands (votes are final).
	effective, err := s.DB.UserVote(r.Context(), g.ID, u.ID)
	if err != nil {
		serverError(w, r, err)
		return
	}

	if cast {
		s.broadcastVoteUpdate(r.Context(), g.ID, q.ID)
	}

	writeJSON(w, 200, map[string]any{"questionId": effective, "cast": cast})
}

// broadcastVoteUpdate pushes the new vote total for a single question to the
// admins watching this game, so the admin console updates live without a
// refetch. It is sent to admins only — players must not see vote counts, or
// the running tally would bias their pick.
func (s *Server) broadcastVoteUpdate(ctx context.Context, gameID, questionID string) {
	count, err := s.DB.VoteCountForQuestion(ctx, questionID)
	if err != nil {
		log.Printf("vote count for question %s: %v", questionID, err)
		return
	}
	s.Hub.BroadcastTo(gameID, map[string]any{
		"type": "voteUpdate",
		"data": map[string]any{"questionId": questionID, "count": count},
	}, func(cl *ws.Client) bool { return cl.Role == ws.RoleAdmin })
}
