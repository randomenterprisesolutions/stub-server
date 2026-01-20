package httpstub

import (
	"log/slog"
	"net/http"
	"sort"
	"sync"
)

// MatchType defines the match types
type MatchType int

const (
	// MatchExact indicates that matching against a stub should use exact string equality.
	// With this match type, the incoming request value must be identical to the stored stub value
	// — no pattern matching or regular expressions are applied. Use MatchRegex for pattern-based matching.
	MatchExact MatchType = iota
	// MatchRegex indicates that matching against a stub should use regular expression pattern matching.
	// With this match type, the stored stub value is treated as a regular expression and the incoming
	// request value must match that pattern. Patterns are interpreted using Go's regexp package;
	// anchor the pattern (e.g., ^...$) if you require a full-string match. Use MatchExact for literal equality.
	MatchRegex
)

// Stub represents a predefined HTTP stub.
type Stub interface {
	Matches(HTTPInvocation) bool
	Invoke(http.ResponseWriter)
	Type() MatchType
}

// Storage is an in-memory storage for HTTP stubs.
type Storage struct {
	stubs []Stub

	m sync.Mutex
}

// NewStorage creates a new instance of Storage.
func NewStorage() *Storage {
	return &Storage{
		stubs: []Stub{},
		m:     sync.Mutex{},
	}
}

// Add adds a new ProtoStub to the storage.
func (p *Storage) Add(s Stub) {
	p.m.Lock()
	defer p.m.Unlock()

	p.stubs = append(p.stubs, s)

	// Sort by Type (Exact < Prefix < Regex < Prefix)
	sort.Slice(p.stubs, func(i, j int) bool {
		return p.stubs[i].Type() < p.stubs[j].Type()
	})
}

// Get retrieves the Output for a given URL and method.
func (p *Storage) Find(inv HTTPInvocation) (Stub, bool) {
	p.m.Lock()
	defer p.m.Unlock()

	var match Stub
	var matches []Stub
	for _, stub := range p.stubs {
		if stub.Matches(inv) {
			if match == nil {
				match = stub
			}
			matches = append(matches, stub)
		}
	}

	if len(matches) > 1 {
		slog.Warn("Multiple stub rules matched",
			slog.String("path", inv.Path),
			slog.String("method", inv.Method),
			slog.Int("matches", len(matches)),
			slog.Any("rules", extractStubInfo(matches)),
		)
	}

	if match == nil {
		return nil, false
	}

	return match, true
}

func extractStubInfo(stubs []Stub) []map[string]any {
	out := make([]map[string]any, 0, len(stubs))
	for _, s := range stubs {
		out = append(out, map[string]any{
			"type": s.Type(),
		})
	}
	return out
}
