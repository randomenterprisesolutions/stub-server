package httpstub

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestHeaderMatcherValidateAndMatches(t *testing.T) {
	cases := []struct {
		name      string
		matcher   HeaderMatcher
		value     string
		wantMatch bool
		wantErr   bool
	}{
		{
			name:      "exact match succeeds",
			matcher:   HeaderMatcher{Exact: "Bearer token123"},
			value:     "Bearer token123",
			wantMatch: true,
		},
		{
			name:      "exact match fails",
			matcher:   HeaderMatcher{Exact: "Bearer token123"},
			value:     "Bearer other",
			wantMatch: false,
		},
		{
			name:      "regex match succeeds",
			matcher:   HeaderMatcher{Regex: `^Bearer .+$`},
			value:     "Bearer sometoken",
			wantMatch: true,
		},
		{
			name:      "regex match fails",
			matcher:   HeaderMatcher{Regex: `^Bearer .+$`},
			value:     "Basic user:pass",
			wantMatch: false,
		},
		{
			name:    "both exact and regex is invalid",
			matcher: HeaderMatcher{Exact: "x", Regex: "x"},
			wantErr: true,
		},
		{
			name:    "neither exact nor regex is invalid",
			matcher: HeaderMatcher{},
			wantErr: true,
		},
		{
			name:    "invalid regex",
			matcher: HeaderMatcher{Regex: "[invalid"},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.matcher.validate()
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantMatch, tc.matcher.matches(tc.value))
		})
	}
}

func TestBodyMatcherValidateAndMatches(t *testing.T) {
	cases := []struct {
		name      string
		matcher   BodyMatcher
		body      map[string]any
		wantMatch bool
		wantErr   bool
	}{
		{
			name:      "exact match succeeds",
			matcher:   BodyMatcher{Exact: map[string]any{"name": "John"}},
			body:      map[string]any{"name": "John"},
			wantMatch: true,
		},
		{
			name:      "exact match fails when extra fields present",
			matcher:   BodyMatcher{Exact: map[string]any{"name": "John"}},
			body:      map[string]any{"name": "John", "age": float64(30)},
			wantMatch: false,
		},
		{
			name:      "contains match succeeds with extra fields",
			matcher:   BodyMatcher{Contains: map[string]any{"name": "John"}},
			body:      map[string]any{"name": "John", "age": float64(30)},
			wantMatch: true,
		},
		{
			name:      "contains match fails when key missing",
			matcher:   BodyMatcher{Contains: map[string]any{"name": "John"}},
			body:      map[string]any{"age": float64(30)},
			wantMatch: false,
		},
		{
			name: "contains match succeeds for nested map",
			matcher: BodyMatcher{Contains: map[string]any{
				"user": map[string]any{"name": "John"},
			}},
			body: map[string]any{
				"user": map[string]any{"name": "John", "age": float64(30)},
			},
			wantMatch: true,
		},
		{
			name:    "both exact and contains is invalid",
			matcher: BodyMatcher{Exact: map[string]any{"a": "b"}, Contains: map[string]any{"a": "b"}},
			wantErr: true,
		},
		{
			name:    "neither exact nor contains is invalid",
			matcher: BodyMatcher{},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.matcher.validate()
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantMatch, tc.matcher.matches(tc.body))
		})
	}
}

func TestRequestMatcherMatches(t *testing.T) {
	cases := []struct {
		name      string
		matcher   RequestMatcher
		inv       HTTPInvocation
		wantMatch bool
	}{
		{
			name: "header exact match succeeds",
			matcher: RequestMatcher{
				Headers: map[string]HeaderMatcher{
					"Authorization": {Exact: "Bearer token"},
				},
			},
			inv: HTTPInvocation{
				Headers: http.Header{"Authorization": []string{"Bearer token"}},
			},
			wantMatch: true,
		},
		{
			name: "header exact match fails",
			matcher: RequestMatcher{
				Headers: map[string]HeaderMatcher{
					"Authorization": {Exact: "Bearer token"},
				},
			},
			inv: HTTPInvocation{
				Headers: http.Header{"Authorization": []string{"Bearer other"}},
			},
			wantMatch: false,
		},
		{
			name: "header regex match succeeds",
			matcher: RequestMatcher{
				Headers: map[string]HeaderMatcher{
					"Content-Type": {Regex: `^application/.*`},
				},
			},
			inv: HTTPInvocation{
				Headers: http.Header{"Content-Type": []string{"application/json"}},
			},
			wantMatch: true,
		},
		{
			name: "missing header fails match",
			matcher: RequestMatcher{
				Headers: map[string]HeaderMatcher{
					"Authorization": {Exact: "Bearer token"},
				},
			},
			inv:       HTTPInvocation{Headers: http.Header{}},
			wantMatch: false,
		},
		{
			name: "body exact match succeeds",
			matcher: RequestMatcher{
				Body: &BodyMatcher{Exact: map[string]any{"key": "value"}},
			},
			inv:       HTTPInvocation{Body: map[string]any{"key": "value"}},
			wantMatch: true,
		},
		{
			name: "body match fails when body is nil",
			matcher: RequestMatcher{
				Body: &BodyMatcher{Exact: map[string]any{"key": "value"}},
			},
			inv:       HTTPInvocation{},
			wantMatch: false,
		},
		{
			name: "body contains match succeeds",
			matcher: RequestMatcher{
				Body: &BodyMatcher{Contains: map[string]any{"key": "value"}},
			},
			inv:       HTTPInvocation{Body: map[string]any{"key": "value", "extra": "data"}},
			wantMatch: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.matcher
			require.NoError(t, m.validate())
			require.Equal(t, tc.wantMatch, m.matches(tc.inv))
		})
	}
}

func TestJSONStubMatchesWithRequestMatcher(t *testing.T) {
	cases := []struct {
		name      string
		stub      JSONStub
		inv       HTTPInvocation
		wantMatch bool
	}{
		{
			name: "header exact match selects stub",
			stub: JSONStub{
				ExactPath:  "/api",
				HTTPMethod: "POST",
				Request: &RequestMatcher{
					Headers: map[string]HeaderMatcher{
						"X-Token": {Exact: "secret"},
					},
				},
				Response: JSONResponse{Status: http.StatusOK},
			},
			inv:       HTTPInvocation{Method: "POST", Path: "/api", Headers: http.Header{"X-Token": []string{"secret"}}},
			wantMatch: true,
		},
		{
			name: "header mismatch rejects stub",
			stub: JSONStub{
				ExactPath:  "/api",
				HTTPMethod: "POST",
				Request: &RequestMatcher{
					Headers: map[string]HeaderMatcher{
						"X-Token": {Exact: "secret"},
					},
				},
				Response: JSONResponse{Status: http.StatusOK},
			},
			inv:       HTTPInvocation{Method: "POST", Path: "/api", Headers: http.Header{"X-Token": []string{"wrong"}}},
			wantMatch: false,
		},
		{
			name: "body contains match selects stub",
			stub: JSONStub{
				ExactPath:  "/submit",
				HTTPMethod: "POST",
				Request: &RequestMatcher{
					Body: &BodyMatcher{Contains: map[string]any{"action": "create"}},
				},
				Response: JSONResponse{Status: http.StatusCreated},
			},
			inv:       HTTPInvocation{Method: "POST", Path: "/submit", Body: map[string]any{"action": "create", "name": "test"}},
			wantMatch: true,
		},
		{
			name: "stub without request matcher matches any request",
			stub: JSONStub{
				ExactPath:  "/open",
				HTTPMethod: "*",
				Response:   JSONResponse{Status: http.StatusOK},
			},
			inv:       HTTPInvocation{Method: "GET", Path: "/open", Headers: http.Header{}},
			wantMatch: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := tc.stub
			require.NoError(t, stub.Validate())
			require.Equal(t, tc.wantMatch, stub.Matches(tc.inv))
		})
	}
}

func TestHandlerRequestMatching(t *testing.T) {
	root := t.TempDir()

	// Stub that matches only requests with a specific header.
	tokenStub := `{
		"path": "/secure",
		"method": "POST",
		"request": {
			"headers": {
				"X-Api-Key": {"exact": "my-key"}
			}
		},
		"response": {"status": 200, "body": {"secured": true}}
	}`
	require.NoError(t, os.WriteFile(filepath.Join(root, "secure.json"), []byte(tokenStub), 0o644))

	handler, err := NewHandler(root)
	require.NoError(t, err)

	// Request with matching header should succeed.
	req := httptest.NewRequest(http.MethodPost, "/secure", strings.NewReader(`{}`))
	req.Header.Set("X-Api-Key", "my-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	// Request with wrong header should return 404.
	req2 := httptest.NewRequest(http.MethodPost, "/secure", strings.NewReader(`{}`))
	req2.Header.Set("X-Api-Key", "wrong-key")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusNotFound, rec2.Code)
}
