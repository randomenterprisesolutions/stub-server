package httpstub

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeStub struct {
	match  bool
	t      MatchType
	called *int
}

func (s fakeStub) Matches(HTTPInvocation) bool { return s.match }
func (s fakeStub) Type() MatchType             { return s.t }
func (s fakeStub) Invoke(http.ResponseWriter)  { (*s.called)++ }

func TestStorageFind_PrioritizesExact(t *testing.T) {
	calls := 0

	regexStub := fakeStub{match: true, t: MatchRegex, called: &calls}
	exactStub := fakeStub{match: true, t: MatchExact, called: &calls}

	cases := []struct {
		name      string
		setup     func(*Storage)
		inv       HTTPInvocation
		wantOK    bool
		wantType  MatchType
	}{
		{
			name:     "match prefers exact",
			setup: func(s *Storage) {
				s.Add(regexStub)
				s.Add(exactStub)
			},
			inv:      HTTPInvocation{Method: "GET", Path: "/any"},
			wantOK:   true,
			wantType: MatchExact,
		},
		{
			name:   "no match returns false",
			setup: func(s *Storage) {
				s.Add(fakeStub{match: false, t: MatchExact, called: new(int)})
			},
			inv:    HTTPInvocation{Method: "GET", Path: "/missing"},
			wantOK: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			storage := NewStorage()
			tc.setup(storage)
			got, ok := storage.Find(tc.inv)
			require.Equal(t, tc.wantOK, ok)
			if tc.wantOK {
				require.Equal(t, tc.wantType, got.Type())
			}
		})
	}
}
