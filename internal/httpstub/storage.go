package httpstub

import (
	"net/http"
	"sync"
)

// Stub represents a predefined HTTP stub.
type Stub interface {
	URL() string
	Method() string
	Write(*http.Request, http.ResponseWriter) error
}

// Storage is an in-memory storage for HTTP stubs.
type Storage struct {
	// represents [URL][Method]
	stubs map[string]map[string]Stub

	m sync.Mutex
}

// NewStorage creates a new instance of Storage.
func NewStorage() *Storage {
	return &Storage{
		stubs: map[string]map[string]Stub{},
		m:     sync.Mutex{},
	}
}

// Add adds a new ProtoStub to the storage.
func (p *Storage) Add(s Stub) {
	p.m.Lock()
	defer p.m.Unlock()

	if p.stubs[s.URL()] == nil {
		p.stubs[s.URL()] = map[string]Stub{}
	}
	p.stubs[s.URL()][s.Method()] = s
}

// Get retrieves the Output for a given URL and method.
func (p *Storage) Get(req *http.Request) (Stub, error) {
	p.m.Lock()
	defer p.m.Unlock()

	matchingURLs, ok := p.stubs[req.URL.Path]
	if !ok {
		return nil, ErrStubNotFound
	}

	stub, ok := matchingURLs[req.Method]
	if !ok {
		return nil, ErrMethodNotAllowed
	}

	return stub, nil
}
