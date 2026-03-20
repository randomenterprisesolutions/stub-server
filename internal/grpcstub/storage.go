package grpcstub

import (
	"sort"
	"sync"
)

// GRPCInvocation captures the metadata and decoded body of a gRPC request for stub matching.
type GRPCInvocation struct {
	// Headers contains the incoming gRPC metadata (keys are lowercase).
	Headers map[string][]string
	// Body is the decoded request message marshalled to a JSON object, or nil if unavailable.
	Body map[string]any
}

// Storage is an in-memory storage for gRPC stubs.
type Storage struct {
	// represents [serviceName][methodName][]ProtoStub
	stubs map[string]map[string][]ProtoStub

	m sync.Mutex
}

var _ Repository = &Storage{}

// NewStorage creates a new instance of Storage.
func NewStorage() *Storage {
	return &Storage{
		stubs: map[string]map[string][]ProtoStub{},
		m:     sync.Mutex{},
	}
}

// Add adds a new ProtoStub to the storage.
// Stubs with a request matcher are stored before stubs without one so that
// more-specific stubs are checked first during Get.
func (p *Storage) Add(s ProtoStub) {
	p.m.Lock()
	defer p.m.Unlock()

	if p.stubs[s.Service] == nil {
		p.stubs[s.Service] = map[string][]ProtoStub{}
	}
	stubs := append(p.stubs[s.Service][s.Method], s)

	// Sort: stubs with a request matcher (more specific) come first.
	sort.SliceStable(stubs, func(i, j int) bool {
		return stubs[i].Request != nil && stubs[j].Request == nil
	})

	p.stubs[s.Service][s.Method] = stubs
}

// Get retrieves the Output for the first stub matching the given service, method, and invocation.
// Stubs without a request matcher act as a fallback and match any invocation.
func (p *Storage) Get(service string, method string, inv GRPCInvocation) (Output, bool) {
	p.m.Lock()
	defer p.m.Unlock()

	stubs, ok := p.stubs[service][method]
	if !ok {
		return Output{}, false
	}

	for _, stub := range stubs {
		if stub.Request == nil || stub.Request.matchesInvocation(inv) {
			return stub.Output, true
		}
	}

	return Output{}, false
}
