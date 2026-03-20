package matchutil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMapContains(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		full      map[string]any
		subset    map[string]any
		wantMatch bool
	}{
		"empty subset always matches": {
			full:      map[string]any{"a": "b"},
			subset:    map[string]any{},
			wantMatch: true,
		},
		"exact key-value match": {
			full:      map[string]any{"a": "b"},
			subset:    map[string]any{"a": "b"},
			wantMatch: true,
		},
		"subset of larger map matches": {
			full:      map[string]any{"a": "b", "c": "d"},
			subset:    map[string]any{"a": "b"},
			wantMatch: true,
		},
		"missing key does not match": {
			full:      map[string]any{"a": "b"},
			subset:    map[string]any{"x": "y"},
			wantMatch: false,
		},
		"wrong value does not match": {
			full:      map[string]any{"a": "b"},
			subset:    map[string]any{"a": "z"},
			wantMatch: false,
		},
		"nested map subset matches": {
			full: map[string]any{
				"user": map[string]any{"name": "John", "age": float64(30)},
			},
			subset: map[string]any{
				"user": map[string]any{"name": "John"},
			},
			wantMatch: true,
		},
		"nested map wrong value does not match": {
			full: map[string]any{
				"user": map[string]any{"name": "John"},
			},
			subset: map[string]any{
				"user": map[string]any{"name": "Jane"},
			},
			wantMatch: false,
		},
		"nested map missing key does not match": {
			full: map[string]any{
				"user": map[string]any{"name": "John"},
			},
			subset: map[string]any{
				"user": map[string]any{"email": "john@example.com"},
			},
			wantMatch: false,
		},
		"full is map but subset value is scalar for same key": {
			full: map[string]any{
				"user": map[string]any{"name": "John"},
			},
			subset: map[string]any{
				"user": "John",
			},
			wantMatch: false,
		},
		"numeric values match": {
			full:      map[string]any{"count": float64(42)},
			subset:    map[string]any{"count": float64(42)},
			wantMatch: true,
		},
		"numeric values differ": {
			full:      map[string]any{"count": float64(42)},
			subset:    map[string]any{"count": float64(0)},
			wantMatch: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tc.wantMatch, MapContains(tc.full, tc.subset))
		})
	}
}
