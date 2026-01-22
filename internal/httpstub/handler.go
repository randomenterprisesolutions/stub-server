// Package httpstub provides an HTTP handler that serves predefined HTTP responses.
package httpstub

import (
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
	inv := HTTPInvocation{
		Method:  r.Method,
		Path:    r.URL.Path,
		Query:   r.URL.Query(),
		Headers: r.Header,
	}

	if r.Body != nil {
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			slog.ErrorContext(r.Context(), "Error reading body", slog.String("error", err.Error()))
			http.Error(w, "Error reading request body", http.StatusInternalServerError)
			return
		}
	}

	stub, ok := s.stubs.Find(inv)
	if ok {
		stub.Invoke(w)
		return
	}

	slog.InfoContext(r.Context(),
		"Stub not found",
		slog.String("path", r.URL.Path),
		slog.String("method", r.Method),
	)
	http.NotFound(w, r)
}
