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

	out, ok := storage.Get("svc", "Get")
	require.True(t, ok)
	require.NotNil(t, out.Code)
	require.Equal(t, code, *out.Code)

	_, ok = storage.Get("svc", "Other")
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
