package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/oglimmer/trivia/backend/internal/auth"
)

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.NotFound(s.notFoundHandler)

	r.Route("/api", func(r chi.Router) {
		// Latency instrumentation is scoped to /api only. /ws is excluded
		// because its "request duration" is the WebSocket session lifetime,
		// which would skew the request-latency histogram into hours.
		// Session lifetime is tracked separately as trivia_ws_session_duration_seconds.
		if s.Metrics != nil {
			r.Use(s.Metrics.InstrumentHTTP)
		}
		r.Get("/version", s.handleVersion)
		r.Post("/admin/login", s.adminLogin)

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAdmin)
			r.Get("/admin/games", s.listGames)
			r.Post("/admin/games", s.createGame)
			r.Get("/admin/users", s.listAllUsers)
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
			r.Post("/admin/games/{code}/questions/import", s.importQuestions)
			r.Post("/admin/games/{code}/questions", s.createPollQuestion)
			r.Put("/admin/games/{code}/questions/{questionId}", s.updatePollQuestion)
			r.Post("/admin/games/{code}/questions/{questionId}/move", s.movePollQuestion)
			r.Get("/admin/games/{code}/votes", s.adminVotes)
		})

		// Image upload / serving — no auth on the read path; UUID is the
		// capability. See docs/image-architecture.md §3.
		r.Post("/images", s.uploadImage)
		r.Get("/images/{id}", s.getImage)
		r.Get("/images/{id}/{variant}", s.getImageVariant)

		// Player-facing endpoints.
		r.Get("/games/{code}", s.getGameForJoin)
		r.Post("/games/{code}/join", s.joinGame)
		r.Get("/me", s.me)
		r.Put("/me", s.updateMe)
		r.Get("/games/{code}/users", s.listUsersPublic)
		r.Get("/games/{code}/questions", s.listQuestionsPublic)
		r.Put("/games/{code}/questions", s.putQuestion)
		r.Get("/games/{code}/leaderboard", s.leaderboard)
		r.Get("/games/{code}/results", s.results)
		r.Get("/games/{code}/myvote", s.myVote)
		r.Post("/games/{code}/vote", s.castVote)
		r.Post("/ai/suggest", s.aiSuggest)
	})

	// WebSocket entry point.
	r.Get("/ws", s.serveWS)

	return r
}
