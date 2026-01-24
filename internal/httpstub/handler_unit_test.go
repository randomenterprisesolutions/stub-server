package httpstub

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type recordingStub struct {
	match  bool
	called bool
	status int
}

func (s *recordingStub) Matches(HTTPInvocation) bool { return s.match }
func (s *recordingStub) Type() MatchType             { return MatchExact }
func (s *recordingStub) Invoke(w http.ResponseWriter) {
	s.called = true
	w.WriteHeader(s.status)
}

func TestHandlerServeHTTP_UsesStub(t *testing.T) {
	storage := NewStorage()
	stub := &recordingStub{match: true, status: http.StatusCreated}
	storage.Add(stub)

	handler := &Handler{stubs: storage}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	handler.ServeHTTP(rec, req)

	require.True(t, stub.called)
	require.Equal(t, http.StatusCreated, rec.Code)
}

func TestHandlerServeHTTP_NotFound(t *testing.T) {
	storage := NewStorage()
	handler := &Handler{stubs: storage}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandlerServeHTTP_BodyReadError(t *testing.T) {
	storage := NewStorage()
	stub := &recordingStub{match: true, status: http.StatusOK}
	storage.Add(stub)

	handler := &Handler{stubs: storage}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/hello", io.NopCloser(errReader{}))

	handler.ServeHTTP(rec, req)

	require.False(t, stub.called)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read error") }
