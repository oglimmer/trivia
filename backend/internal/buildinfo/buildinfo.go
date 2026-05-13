// Package buildinfo holds build-time identification populated via -ldflags.
package buildinfo

// Populated at build time via:
//
//	-ldflags "-X github.com/oglimmer/trivia/backend/internal/buildinfo.Version=...
//	          -X github.com/oglimmer/trivia/backend/internal/buildinfo.Commit=...
//	          -X github.com/oglimmer/trivia/backend/internal/buildinfo.Time=..."
var (
	Version = "dev"
	Commit  = "unknown"
	Time    = "unknown"
)
