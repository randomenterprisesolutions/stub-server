// Package handler provides an HTTP handler that can route requests to either an
// HTTP stub server or a gRPC stub server based on the request properties.
package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/randomenterprisesolutions/stub-server/internal/grpcstub"
	"github.com/randomenterprisesolutions/stub-server/internal/httpstub"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
)

// Server represents a server that can handle both HTTP and gRPC requests.
type Server struct {
	grpcServer  *grpc.Server
	httpHandler http.Handler
}

var _ http.Handler = &Server{}

// WithProto configures the server to handle gRPC requests using the provided
// proto and stub directories.
func (s *Server) WithProto(protoDir string, stubDir string) error {
	server, err := grpcstub.NewServer(protoDir, stubDir)
	if err != nil {
		return fmt.Errorf("initialize gRPC server: %w", err)
	}

	s.grpcServer = server

	return nil
}

// WithHTTP configures the server to handle HTTP requests using the provided
// HTTP stubs directory.
func (s *Server) WithHTTP(httpStubs string) error {
	handler, err := httpstub.NewHandler(httpStubs)
	if err != nil {
		return fmt.Errorf("initialize HTTP handler: %w", err)
	}

	s.httpHandler = handler

	return nil
}

// ServeHTTP routes incoming HTTP requests to either the gRPC server or the HTTP
// handler based on the request properties. If the request is a gRPC
// request (HTTP/2 with "application/grpc" content type), it is forwarded to the
// gRPC server. Otherwise, it is handled by the HTTP handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.ProtoMajor == 2 && strings.HasPrefix(
		r.Header.Get("Content-Type"), "application/grpc") {
		if s.grpcServer == nil {
			slog.ErrorContext(r.Context(), "No gRPC stub server configured")
			http.Error(w, "No gRPC stub server configured", http.StatusNotImplemented)
			return
		}
		s.grpcServer.ServeHTTP(w, r)
		return
	}

	if s.httpHandler == nil {
		slog.ErrorContext(r.Context(), "No HTTP stub server configured")
		http.Error(w, "No HTTP stub server configured", http.StatusNotImplemented)
		return
	}

	s.httpHandler.ServeHTTP(w, r)
}

func allowH2c(next http.Handler) http.Handler {
	h2server := &http2.Server{IdleTimeout: time.Second * 60}
	return h2c.NewHandler(next, h2server)
}

// New creates a new Server instance and configures it based on the provided
// directories for HTTP stubs, proto files, and gRPC stubs. If the respective
// directory is an empty string, that type of handling is not configured.
func New(httpStubDir string, protoDir string, protoStubDir string) (http.Handler, error) {
	mux := http.NewServeMux()

	s := &Server{}

	mux.Handle("/", s)

	if httpStubDir != "" {
		if err := s.WithHTTP(httpStubDir); err != nil {
			return nil, fmt.Errorf("create HTTP handler: %w", err)
		}
	}

	if protoDir != "" && protoStubDir != "" {
		if err := s.WithProto(protoDir, protoStubDir); err != nil {
			return nil, fmt.Errorf("create gRPC handler: %w", err)
		}
	}

	return allowH2c(s), nil
}
