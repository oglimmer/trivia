package api

import (
	"net/http"

	"github.com/oglimmer/trivia/backend/internal/buildinfo"
)

type buildInfoResp struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	GitCommit string `json:"gitCommit"`
	BuildTime string `json:"buildTime"`
}

func (s *Server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, buildInfoResp{
		Name:      "trivia-backend",
		Version:   buildinfo.Version,
		GitCommit: buildinfo.Commit,
		BuildTime: buildinfo.Time,
	})
}
