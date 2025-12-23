// Package httpstub provides an HTTP handler that serves predefined HTTP responses.
package httpstub

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

// Handler is an HTTP handler that serves predefined HTTP stubs.
type Handler struct {
	stubs *Storage
}

var _ http.Handler = &Handler{}

// NewHandler creates a new Handler by loading HTTP stubs from the specified directory.
func NewHandler(stubDir string) (*Handler, error) {
	storage := NewStorage()
	if err := loadStubs(stubDir, storage); err != nil {
		return nil, fmt.Errorf("load HTTP stubs from %v: %w ", stubDir, err)
	}

	return &Handler{
		stubs: storage,
	}, nil
}

// ServeHTTP serves HTTP requests based on the loaded stubs.
func (s *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	stub, err := s.stubs.Get(r)
	if err != nil {
		slog.ErrorContext(r.Context(),
			"Could not get stub",
			slog.String("path", r.URL.Path),
			slog.String("method", r.Method),
			slog.String("error", err.Error()),
		)
		if errors.Is(err, ErrStubNotFound) {
			http.Error(w, "unknown stub", http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrMethodNotAllowed) {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		http.Error(w, "unknown stub", http.StatusInternalServerError)
		return
	}

	if r.Body != nil {
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			slog.ErrorContext(r.Context(), "Error reading body", slog.String("error", err.Error()))
			http.Error(w, "Error reading request body", http.StatusInternalServerError)
			return
		}
	}

	if err := stub.Write(r, w); err != nil {
		slog.ErrorContext(r.Context(), "Failed to write response", slog.String("error", err.Error()))
		http.Error(w, "Failed to write response", http.StatusInternalServerError)
		return
	}
}
