package grpcstub

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

func TestStorageAddGet(t *testing.T) {
	storage := NewStorage()
	code := codes.NotFound
	storage.Add(ProtoStub{
		Service: "svc",
		Method:  "Get",
		Output: Output{
			Code:  &code,
			Error: "missing",
		},
	})

	out, ok := storage.Find("svc", "Get", GRPCInvocation{})
	require.True(t, ok)
	require.NotNil(t, out.Code)
	require.Equal(t, code, *out.Code)

	_, ok = storage.Find("svc", "Other", GRPCInvocation{})
	require.False(t, ok)
}

func TestStreamValidate(t *testing.T) {
	cases := []struct {
		name    string
		stream  Stream
		wantErr bool
	}{
		{
			name:    "empty stream",
			stream:  Stream{},
			wantErr: true,
		},
		{
			name:   "data stream",
			stream: Stream{Data: []json.RawMessage{json.RawMessage(`{"ok":true}`)}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.stream.validate()
			require.Equal(t, tc.wantErr, err != nil)
		})
	}
}

func TestOutputValidate(t *testing.T) {
	cases := []struct {
		name    string
		output  Output
		wantErr bool
	}{
		{
			name:    "empty output",
			output:  Output{},
			wantErr: true,
		},
		{
			name:   "error output",
			output: Output{Error: "boom"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.output.validate()
			require.Equal(t, tc.wantErr, err != nil)
		})
	}
}

func TestProtoStubValidate(t *testing.T) {
	cases := []struct {
		name    string
		stub    ProtoStub
		wantErr bool
	}{
		{
			name:    "missing required fields",
			stub:    ProtoStub{},
			wantErr: true,
		},
		{
			name: "valid stub",
			stub: ProtoStub{
				Service: "svc",
				Method:  "Get",
				Output: Output{
					Error: "boom",
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.stub.validate()
			require.Equal(t, tc.wantErr, err != nil)
		})
	}
}

func TestLoadFile(t *testing.T) {
	root := t.TempDir()
	stubPath := filepath.Join(root, "stub.json")
	payload := `{
  "service": "svc",
  "method": "Get",
  "output": {"error": "boom"}
}`
	require.NoError(t, os.WriteFile(stubPath, []byte(payload), 0o644))

	stub, err := loadFile(stubPath)
	require.NoError(t, err)
	require.Equal(t, "svc", stub.Service)
	require.Equal(t, "Get", stub.Method)
	require.Equal(t, "boom", stub.Output.Error)
}

func TestStorageFindWithRequestMatcher(t *testing.T) {
	storage := NewStorage()

	// Specific stub: only matches when header "x-token" equals "secret".
	storage.Add(ProtoStub{
		Service: "svc",
		Method:  "Call",
		Request: &RequestMatcher{
			Headers: map[string]HeaderMatcher{
				"x-token": {Exact: "secret"},
			},
		},
		Output: Output{Error: "matched-specific"},
	})

	// Fallback stub: matches any invocation.
	storage.Add(ProtoStub{
		Service: "svc",
		Method:  "Call",
		Output:  Output{Error: "matched-fallback"},
	})

	// Invocation with matching header should return the specific stub.
	out, ok := storage.Find("svc", "Call", GRPCInvocation{
		Headers: map[string][]string{"x-token": {"secret"}},
	})
	require.True(t, ok)
	require.Equal(t, "matched-specific", out.Error)

	// Invocation without the header should fall through to the fallback.
	out, ok = storage.Find("svc", "Call", GRPCInvocation{
		Headers: map[string][]string{},
	})
	require.True(t, ok)
	require.Equal(t, "matched-fallback", out.Error)

	// Unknown method returns nothing.
	_, ok = storage.Find("svc", "Unknown", GRPCInvocation{})
	require.False(t, ok)
}

func TestStorageFindBodyMatcher(t *testing.T) {
	storage := NewStorage()

	storage.Add(ProtoStub{
		Service: "svc",
		Method:  "Call",
		Request: &RequestMatcher{
			Body: &BodyMatcher{Contains: map[string]any{"action": "delete"}},
		},
		Output: Output{Error: "delete-stub"},
	})
	storage.Add(ProtoStub{
		Service: "svc",
		Method:  "Call",
		Output:  Output{Error: "fallback-stub"},
	})

	// Matching body returns specific stub.
	out, ok := storage.Find("svc", "Call", GRPCInvocation{
		Body: map[string]any{"action": "delete", "id": "123"},
	})
	require.True(t, ok)
	require.Equal(t, "delete-stub", out.Error)

	// Non-matching body falls through to fallback.
	out, ok = storage.Find("svc", "Call", GRPCInvocation{
		Body: map[string]any{"action": "create"},
	})
	require.True(t, ok)
	require.Equal(t, "fallback-stub", out.Error)
}

func TestRequestMatcherValidate(t *testing.T) {
	cases := []struct {
		name    string
		matcher RequestMatcher
		wantErr bool
	}{
		{
			name: "valid header exact matcher",
			matcher: RequestMatcher{
				Headers: map[string]HeaderMatcher{
					"authorization": {Exact: "Bearer token"},
				},
			},
		},
		{
			name: "valid header regex matcher",
			matcher: RequestMatcher{
				Headers: map[string]HeaderMatcher{
					"content-type": {Regex: `^application/.*`},
				},
			},
		},
		{
			name: "invalid regex in header matcher",
			matcher: RequestMatcher{
				Headers: map[string]HeaderMatcher{
					"x-header": {Regex: "[invalid"},
				},
			},
			wantErr: true,
		},
		{
			name: "valid body contains matcher",
			matcher: RequestMatcher{
				Body: &BodyMatcher{Contains: map[string]any{"key": "value"}},
			},
		},
		{
			name: "body with both exact and contains is invalid",
			matcher: RequestMatcher{
				Body: &BodyMatcher{
					Exact:    map[string]any{"a": "b"},
					Contains: map[string]any{"a": "b"},
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.matcher
			err := m.validate()
			require.Equal(t, tc.wantErr, err != nil)
		})
	}
}
