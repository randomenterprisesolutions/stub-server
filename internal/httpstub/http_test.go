package httpstub

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRawHTTPStubLazyLoad(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	stubDir := filepath.Join(root, "echo", "GET")
	require.NoError(t, os.MkdirAll(stubDir, 0o755))

	stubPath := filepath.Join(stubDir, "test.http")
	require.NoError(t, os.WriteFile(stubPath, []byte(rawHTTPResponse("Some text")), 0o644))

	handler, err := NewHandler(root)
	require.NoError(t, err)

	resp := serveGet(t, handler, "/echo")
	require.Equal(t, http.StatusOK, resp.status)
	require.Equal(t, "Some text", resp.body)

	// Lazy-loading check: change the on-disk stub after the first request and expect
	// the next request to reflect the new file contents.
	require.NoError(t, os.WriteFile(stubPath, []byte(rawHTTPResponse("Changed")), 0o644))

	resp = serveGet(t, handler, "/echo")
	require.Equal(t, http.StatusOK, resp.status)
	require.Equal(t, "Changed", resp.body)
}

type httpResponse struct {
	status int
	body   string
}

func serveGet(t *testing.T, handler http.Handler, path string) httpResponse {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)

	return httpResponse{status: rec.Code, body: string(body)}
}

func rawHTTPResponse(body string) string {
	return fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Length: %d\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s", len(body), body)
}
