package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/oglimmer/trivia/backend/internal/images"
)

// imageCacheControl: bytes for a given id never change (content-addressed), so
// browsers and any CDN can cache forever. Stale entries are an impossibility.
const imageCacheControl = "public, max-age=31536000, immutable"

func (s *Server) uploadImage(w http.ResponseWriter, r *http.Request) {
	if s.Images == nil {
		writeErr(w, http.StatusServiceUnavailable, "images not configured")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, images.MaxUploadBytes)
	f, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "expected multipart field 'file'")
		return
	}
	defer func() { _ = f.Close() }()

	id, err := s.Images.Store(r.Context(), f)
	if err != nil {
		if errors.Is(err, images.ErrTooLarge) {
			writeErr(w, http.StatusRequestEntityTooLarge, err.Error())
			return
		}
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if s.Metrics != nil {
		s.Metrics.ImagesUploaded.Inc()
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id})
}

func (s *Server) getImage(w http.ResponseWriter, r *http.Request) {
	if s.Images == nil {
		writeErr(w, http.StatusServiceUnavailable, "images not configured")
		return
	}
	id := chi.URLParam(r, "id")
	blob, err := s.Images.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, images.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	serveBlob(w, r, blob)
}

func (s *Server) getImageVariant(w http.ResponseWriter, r *http.Request) {
	if s.Images == nil {
		writeErr(w, http.StatusServiceUnavailable, "images not configured")
		return
	}
	id := chi.URLParam(r, "id")
	kind := chi.URLParam(r, "variant")
	if !images.ValidVariant(kind) {
		writeErr(w, http.StatusNotFound, "unknown variant")
		return
	}
	blob, err := s.Images.GetVariant(r.Context(), id, kind)
	if err != nil {
		if errors.Is(err, images.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	serveBlob(w, r, blob)
}

func serveBlob(w http.ResponseWriter, r *http.Request, b *images.Blob) {
	w.Header().Set("ETag", b.ETag)
	w.Header().Set("Cache-Control", imageCacheControl)
	if match := r.Header.Get("If-None-Match"); match != "" && match == b.ETag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", b.Mime)
	_, _ = w.Write(b.Bytes)
}
