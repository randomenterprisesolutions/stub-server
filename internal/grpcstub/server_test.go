package grpcstub

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseGRPCMethod(t *testing.T) {
	cases := []struct {
		name       string
		input      string
		wantErr    bool
		wantSvc    string
		wantMethod string
	}{
		{
			name:       "valid method",
			input:      "/helloworld.Greeter/SayHello",
			wantSvc:    "helloworld.Greeter",
			wantMethod: "SayHello",
		},
		{
			name:    "missing slashes",
			input:   "missing/slashes",
			wantErr: true,
		},
		{
			name:    "missing method",
			input:   "/service/",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, method, err := parseGRPCMethod(tc.input)
			require.Equal(t, tc.wantErr, err != nil)
			if !tc.wantErr {
				require.Equal(t, tc.wantSvc, service)
				require.Equal(t, tc.wantMethod, method)
			}
		})
	}
}
