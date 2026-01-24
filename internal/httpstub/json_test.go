package httpstub

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJSONStubValidateAndMatches(t *testing.T) {
	cases := []struct {
		name          string
		stub          JSONStub
		inv           HTTPInvocation
		wantMatch     bool
		wantRegexInit bool
	}{
		{
			name: "exact path matches method",
			stub: JSONStub{
				ExactPath:  "/hello",
				HTTPMethod: "GET",
				Response: JSONResponse{
					Status: http.StatusOK,
				},
			},
			inv:       HTTPInvocation{Method: "GET", Path: "/hello"},
			wantMatch: true,
		},
		{
			name: "exact path rejects method",
			stub: JSONStub{
				ExactPath:  "/hello",
				HTTPMethod: "GET",
				Response: JSONResponse{
					Status: http.StatusOK,
				},
			},
			inv:       HTTPInvocation{Method: "POST", Path: "/hello"},
			wantMatch: false,
		},
		{
			name: "regex path matches",
			stub: JSONStub{
				RegexPath:  "^/users/[0-9]+$",
				HTTPMethod: "*",
				Response: JSONResponse{
					Status: http.StatusOK,
				},
			},
			inv:           HTTPInvocation{Method: "POST", Path: "/users/42"},
			wantMatch:     true,
			wantRegexInit: true,
		},
		{
			name: "regex path rejects",
			stub: JSONStub{
				RegexPath:  "^/users/[0-9]+$",
				HTTPMethod: "*",
				Response: JSONResponse{
					Status: http.StatusOK,
				},
			},
			inv:           HTTPInvocation{Method: "POST", Path: "/users/abc"},
			wantMatch:     false,
			wantRegexInit: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := tc.stub
			require.NoError(t, stub.Validate())
			require.Equal(t, tc.wantRegexInit, stub.regex != nil)
			require.Equal(t, tc.wantMatch, stub.Matches(tc.inv))
		})
	}
}

func TestJSONStubValidate_Errors(t *testing.T) {
	cases := []struct {
		name string
		stub JSONStub
	}{
		{
			name: "missing method",
			stub: JSONStub{
				ExactPath: "/hello",
				Response: JSONResponse{
					Status: http.StatusOK,
				},
			},
		},
		{
			name: "missing path and regex",
			stub: JSONStub{
				HTTPMethod: "GET",
				Response: JSONResponse{
					Status: http.StatusOK,
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Error(t, tc.stub.Validate())
		})
	}
}

func TestJSONResponseWrite(t *testing.T) {
	resp := JSONResponse{
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Status: http.StatusCreated,
		Body: map[string]any{
			"ok": true,
		},
	}

	rec := httptest.NewRecorder()
	require.NoError(t, resp.Write(rec))

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	require.JSONEq(t, `{"ok": true}`, rec.Body.String())
}

func TestJSONResponseValidate(t *testing.T) {
	resp := JSONResponse{
		Status: 42,
	}
	require.Error(t, resp.Validate())
}

func TestLoadJSONFile(t *testing.T) {
	root := t.TempDir()
	stubPath := filepath.Join(root, "stub.json")
	payload := `{
  "path": "/hello",
  "method": "GET",
  "response": {
    "status": 200,
    "body": {"message": "ok"}
  }
}`
	require.NoError(t, os.WriteFile(stubPath, []byte(payload), 0o644))

	stub, err := loadJSONFile(root, stubPath)
	require.NoError(t, err)

	jsonStub := stub.(JSONStub)
	require.Equal(t, "/hello", jsonStub.ExactPath)
	require.Equal(t, "GET", jsonStub.HTTPMethod)
	require.Equal(t, http.StatusOK, jsonStub.Response.Status)
	require.Equal(t, "ok", jsonStub.Response.Body["message"])
}
