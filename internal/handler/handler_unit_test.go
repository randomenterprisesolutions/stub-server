package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServerServeHTTP_NoHTTPConfigured(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotImplemented, rec.Code)
}

func TestServerServeHTTP_NoGRPCConfigured(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest(http.MethodPost, "/svc.Method", nil)
	req.ProtoMajor = 2
	req.Header.Set("Content-Type", "application/grpc")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotImplemented, rec.Code)
}

func TestServerServeHTTP_UsesHTTPHandler(t *testing.T) {
	called := false
	server := &Server{
		httpHandler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusAccepted)
		}),
	}

	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	require.True(t, called)
	require.Equal(t, http.StatusAccepted, rec.Code)
}
