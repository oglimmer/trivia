package api

import (
	_ "embed"
	"html/template"
	"net/http"
	"strings"
)

//go:embed notfound.html
var notFoundHTML string

var notFoundTmpl = template.Must(template.New("404").Parse(notFoundHTML))

// notFoundHandler is wired as chi's NotFound. Clients that accept HTML (i.e.
// browsers) get a styled page matching the frontend's 404; everything else
// gets a JSON error so API/WS callers can parse it.
func (s *Server) notFoundHandler(w http.ResponseWriter, r *http.Request) {
	if !acceptsHTML(r) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_ = notFoundTmpl.Execute(w, struct{ Path string }{Path: r.URL.Path})
}

func acceptsHTML(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}
