package httpstub

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPStubMatches(t *testing.T) {
	stub := &HTTPStub{
		Path:       "/echo",
		HTTPMethod: "GET",
	}

	cases := []struct {
		name  string
		inv   HTTPInvocation
		match bool
	}{
		{
			name:  "method matches",
			inv:   HTTPInvocation{Method: "GET", Path: "/echo"},
			match: true,
		},
		{
			name:  "method mismatch",
			inv:   HTTPInvocation{Method: "POST", Path: "/echo"},
			match: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.match, stub.Matches(tc.inv))
		})
	}
}

func TestHTTPStubValidate(t *testing.T) {
	cases := []struct {
		name string
		stub *HTTPStub
	}{
		{
			name: "missing everything",
			stub: &HTTPStub{},
		},
		{
			name: "missing method",
			stub: &HTTPStub{Path: "/echo"},
		},
		{
			name: "missing response path",
			stub: &HTTPStub{Path: "/echo", HTTPMethod: "GET"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Error(t, tc.stub.Validate())
		})
	}
}

func TestLoadHTTPFile(t *testing.T) {
	root := t.TempDir()
	stubDir := filepath.Join(root, "echo", "GET")
	require.NoError(t, os.MkdirAll(stubDir, 0o755))

	stubPath := filepath.Join(stubDir, "test.http")
	require.NoError(t, os.WriteFile(stubPath, []byte(rawHTTPResponse("Hello")), 0o644))

	got, err := loadHTTPFile(root, stubPath)
	require.NoError(t, err)

	httpStub := got.(*HTTPStub)
	require.Equal(t, "/echo", httpStub.Path)
	require.Equal(t, "GET", httpStub.HTTPMethod)
	require.Equal(t, stubPath, httpStub.ResponsePath)
	require.Equal(t, MatchExact, httpStub.Type())
	require.True(t, httpStub.Matches(HTTPInvocation{Method: "GET", Path: "/echo"}))
}

func TestHTTPStubInvoke_InvalidFile(t *testing.T) {
	stub := &HTTPStub{
		Path:         "/echo",
		HTTPMethod:   "GET",
		ResponsePath: filepath.Join(t.TempDir(), "missing.http"),
	}

	rec := httptest.NewRecorder()
	stub.Invoke(rec)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
}
