package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/oglimmer/trivia/backend/internal/auth"
)

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })

	r.Route("/api", func(r chi.Router) {
		r.Get("/version", s.handleVersion)
		r.Post("/admin/login", s.adminLogin)

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAdmin)
			r.Get("/admin/games", s.listGames)
			r.Post("/admin/games", s.createGame)
			r.Get("/admin/games/{code}", s.adminGame)
			r.Delete("/admin/games/{code}", s.deleteGame)
			r.Post("/admin/games/{code}/state", s.setGameState)
			r.Put("/admin/games/{code}/settings", s.updateGameSettings)
			r.Post("/admin/games/{code}/activate", s.activateQuestion)
			r.Post("/admin/games/{code}/reveal", s.revealQuestion)
			r.Post("/admin/games/{code}/next", s.nextQuestion)
			r.Post("/admin/games/{code}/finish", s.finishGame)
			r.Delete("/admin/games/{code}/users/{userId}", s.deleteUser)
			r.Get("/admin/games/{code}/users/{userId}/impersonate", s.impersonateUser)
			r.Delete("/admin/games/{code}/questions/{questionId}", s.deleteQuestion)
		})

		// Player-facing endpoints.
		r.Get("/games/{code}", s.getGameForJoin)
		r.Post("/games/{code}/join", s.joinGame)
		r.Get("/me", s.me)
		r.Put("/me", s.updateMe)
		r.Get("/games/{code}/users", s.listUsersPublic)
		r.Get("/games/{code}/questions", s.listQuestionsPublic)
		r.Put("/games/{code}/questions", s.putQuestion)
		r.Get("/games/{code}/leaderboard", s.leaderboard)
		r.Post("/ai/suggest", s.aiSuggest)
	})

	// WebSocket entry point.
	r.Get("/ws", s.serveWS)

	return r
}
