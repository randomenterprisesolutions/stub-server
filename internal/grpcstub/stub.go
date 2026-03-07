package grpcstub

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"google.golang.org/grpc/codes"

	"github.com/randomenterprisesolutions/stub-server/internal/matchutil"
)

// HeaderMatcher matches a single gRPC metadata value using exact string equality or a regex pattern.
type HeaderMatcher struct {
	Exact string `json:"exact"`
	Regex string `json:"regex"`
	regex *regexp.Regexp
}

func (m *HeaderMatcher) matches(value string) bool {
	if m.Exact != "" {
		return value == m.Exact
	}
	if m.regex != nil {
		return m.regex.MatchString(value)
	}
	return false
}

func (m *HeaderMatcher) validate() error {
	if m.Exact != "" && m.Regex != "" {
		return errors.New(`only one of "exact" or "regex" can be set`)
	}
	if m.Exact == "" && m.Regex == "" {
		return errors.New(`one of "exact" or "regex" is required`)
	}
	if m.Regex != "" {
		compiled, err := regexp.Compile(m.Regex)
		if err != nil {
			return fmt.Errorf("compile regex: %w", err)
		}
		m.regex = compiled
	}
	return nil
}

// BodyMatcher matches a JSON-encoded gRPC request body against an exact map or a contains (subset) map.
type BodyMatcher struct {
	Exact    map[string]any `json:"exact"`
	Contains map[string]any `json:"contains"`
}

func (m *BodyMatcher) matches(body map[string]any) bool {
	if m.Exact != nil {
		return reflect.DeepEqual(body, m.Exact)
	}
	if m.Contains != nil {
		return matchutil.MapContains(body, m.Contains)
	}
	return false
}

func (m *BodyMatcher) validate() error {
	if m.Exact != nil && m.Contains != nil {
		return errors.New(`only one of "exact" or "contains" can be set`)
	}
	if m.Exact == nil && m.Contains == nil {
		return errors.New(`one of "exact" or "contains" is required`)
	}
	return nil
}

// RequestMatcher holds matchers for gRPC request attributes used to select a stub.
type RequestMatcher struct {
	Headers map[string]HeaderMatcher `json:"headers"`
	Body    *BodyMatcher             `json:"body"`
}

func (m *RequestMatcher) matchesInvocation(inv GRPCInvocation) bool {
	for name, matcher := range m.Headers {
		values := inv.Headers[strings.ToLower(name)]
		if len(values) == 0 {
			return false
		}
		matched := false
		for _, v := range values {
			if matcher.matches(v) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if m.Body != nil {
		if inv.Body == nil {
			return false
		}
		if !m.Body.matches(inv.Body) {
			return false
		}
	}
	return true
}

func (m *RequestMatcher) validate() error {
	for name, matcher := range m.Headers {
		if err := matcher.validate(); err != nil {
			return fmt.Errorf("header %q: %w", name, err)
		}
		// Reassign to preserve the compiled regex stored during validate().
		m.Headers[name] = matcher
	}
	if m.Body != nil {
		if err := m.Body.validate(); err != nil {
			return fmt.Errorf("body: %w", err)
		}
	}
	return nil
}

// Stream represents a stream of gRPC responses.
type Stream struct {
	Data  []json.RawMessage `json:"data"`
	Error string            `json:"error"`
	Code  *codes.Code       `json:"code,omitempty"`
	Delay int               `json:"delay,omitempty"`
}

func (s *Stream) validate() error {
	if s.Code == nil && len(s.Data) == 0 && s.Error == "" {
		return fmt.Errorf(`stream can't be empty`)
	}
	return nil
}

// Output represents the output of a gRPC method, which can be a single response or a stream.
type Output struct {
	Data   json.RawMessage `json:"data"`
	Error  string          `json:"error"`
	Code   *codes.Code     `json:"code,omitempty"`
	Stream *Stream         `json:"stream"`
}

func (o *Output) validate() error {
	if o.Code == nil && o.Data == nil && o.Error == "" && o.Stream == nil {
		return fmt.Errorf(`output can't be empty`)
	}

	if o.Stream != nil {
		return o.Stream.validate()
	}
	return nil
}

// ProtoStub represents a gRPC stub definition.
type ProtoStub struct {
	Service string          `json:"service"`
	Method  string          `json:"method"`
	Request *RequestMatcher `json:"request"`
	Output  Output          `json:"output"`
}

func (s *ProtoStub) validate() error {
	if s.Service == "" {
		return fmt.Errorf(`"service" field is required`)
	}
	if s.Method == "" {
		return fmt.Errorf(`"method" field is required`)
	}

	if s.Request != nil {
		if err := s.Request.validate(); err != nil {
			return fmt.Errorf("request validation: %w", err)
		}
	}

	return s.Output.validate()
}

func (s *GRPCService) loadStubs(dir string) error {
	stubs, err := load(dir)
	if err != nil {
		return fmt.Errorf("load stubs: %w", err)
	}

	for _, stub := range stubs {
		if s.sdMap[stub.Service] == nil {
			return fmt.Errorf(`no service "%v" registered`, stub.Service)
		}
		s.stubs.Add(stub)
	}

	return nil
}

func load(dir string) ([]ProtoStub, error) {
	stubs := make([]ProtoStub, 0)
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			if filepath.Ext(path) != ".json" {
				return nil
			}

			stub, err := loadFile(path)
			if err != nil {
				return fmt.Errorf("load stub from file %v: %w", path, err)
			}

			stubs = append(stubs, stub)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf(`read dir "%v": %w`, dir, err)
	}
	return stubs, nil
}

func loadFile(path string) (s ProtoStub, err error) {
	f, err := os.Open(path)
	if err != nil {
		return ProtoStub{}, fmt.Errorf("open file: %v: %w", path, err)
	}
	defer func() {
		closeErr := f.Close()
		if closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close file: %w", closeErr))
		}
	}()

	var stub ProtoStub
	if err := json.NewDecoder(f).Decode(&stub); err != nil {
		return ProtoStub{}, fmt.Errorf("unmarshal stub %v: %w", path, err)
	}

	if err := stub.validate(); err != nil {
		return ProtoStub{}, fmt.Errorf("stub validation %v: %w", path, err)
	}
	return stub, nil
}
