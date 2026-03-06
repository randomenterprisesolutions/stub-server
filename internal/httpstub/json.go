package httpstub

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
)

// HeaderMatcher matches a single request header value using exact string equality or a regex pattern.
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
			return fmt.Errorf("invalid regex: %w", err)
		}
		m.regex = compiled
	}
	return nil
}

// BodyMatcher matches a JSON request body against an exact map or a contains (subset) map.
type BodyMatcher struct {
	Exact    map[string]any `json:"exact"`
	Contains map[string]any `json:"contains"`
}

func (m *BodyMatcher) matches(body map[string]any) bool {
	if m.Exact != nil {
		return reflect.DeepEqual(body, m.Exact)
	}
	if m.Contains != nil {
		return jsonContains(body, m.Contains)
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

// RequestMatcher holds matchers for HTTP request attributes used to select a stub.
type RequestMatcher struct {
	Headers map[string]HeaderMatcher `json:"headers"`
	Body    *BodyMatcher             `json:"body"`
}

func (m *RequestMatcher) matches(inv HTTPInvocation) bool {
	for name, matcher := range m.Headers {
		if !matcher.matches(inv.Headers.Get(name)) {
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

// jsonContains reports whether full contains all key-value pairs from subset, recursively for nested maps.
func jsonContains(full, subset map[string]any) bool {
	for k, sv := range subset {
		fv, ok := full[k]
		if !ok {
			return false
		}
		svMap, svIsMap := sv.(map[string]any)
		fvMap, fvIsMap := fv.(map[string]any)
		if svIsMap && fvIsMap {
			if !jsonContains(fvMap, svMap) {
				return false
			}
		} else if !reflect.DeepEqual(fv, sv) {
			return false
		}
	}
	return true
}

// JSONStub represents a predefined HTTP stub.
type JSONStub struct {
	ExactPath  string          `json:"path"`
	RegexPath  string          `json:"regex"`
	HTTPMethod string          `json:"method"`
	Request    *RequestMatcher `json:"request"`
	Response   JSONResponse    `json:"response"`
	regex      *regexp.Regexp
}

var _ Stub = &JSONStub{}

// Matches checks if the JSONStub matches the given HTTP request.
func (s JSONStub) Matches(inv HTTPInvocation) bool {
	if s.ExactPath != "" {
		if s.HTTPMethod != "*" && inv.Method != s.HTTPMethod {
			return false
		}
		if inv.Path != s.ExactPath {
			return false
		}
	} else {
		if s.HTTPMethod != "" && s.HTTPMethod != "*" && inv.Method != s.HTTPMethod {
			return false
		}
		var pathMatches bool
		if s.regex != nil {
			pathMatches = s.regex.MatchString(inv.Path)
		} else {
			pathMatches, _ = regexp.MatchString(s.RegexPath, inv.Path)
		}
		if !pathMatches {
			return false
		}
	}

	if s.Request != nil && !s.Request.matches(inv) {
		return false
	}

	return true
}

// Invoke writes the JSONStub response to the provided http.ResponseWriter.
func (s JSONStub) Invoke(w http.ResponseWriter) {
	if err := s.Response.Write(w); err != nil {
		http.Error(w, "Failed to write response", http.StatusInternalServerError)
	}
}

// Type returns the MatchType
func (s JSONStub) Type() MatchType {
	if s.ExactPath != "" {
		return MatchExact
	}
	return MatchRegex
}

// Validate validates the JSONStub fields.
func (s *JSONStub) Validate() error {
	if (s.ExactPath == "" && s.RegexPath == "") ||
		(s.ExactPath != "" && s.RegexPath != "") {
		return errors.New(`either "path" or "regex" field is required`)
	}

	if s.HTTPMethod == "" {
		return errors.New(`"method" field is required`)
	}

	if s.RegexPath != "" {
		compiled, err := regexp.Compile(s.RegexPath)
		if err != nil {
			return fmt.Errorf("invalid regex: %w", err)
		}
		s.regex = compiled
	}

	if s.Request != nil {
		if err := s.Request.validate(); err != nil {
			return fmt.Errorf("request validation: %w", err)
		}
	}

	if err := s.Response.Validate(); err != nil {
		return fmt.Errorf("response validation: %w", err)
	}

	return nil
}

// JSONResponse represents an HTTP response defined in a stub.
type JSONResponse struct {
	Header http.Header    `json:"header"`
	Body   map[string]any `json:"body"`
	Status int            `json:"status"`
}

// Write writes the JSONResponse to the provided http.ResponseWriter.
func (r JSONResponse) Write(w http.ResponseWriter) error {
	for k, val := range r.Header {
		for _, v := range val {
			w.Header().Set(k, v)
		}
	}

	w.WriteHeader(r.Status)

	if r.Body == nil {
		return nil
	}

	if err := json.NewEncoder(w).Encode(r.Body); err != nil {
		return fmt.Errorf("encode body: %w", err)
	}

	return nil
}

// Validate validates the JSONResponse fields.
func (r JSONResponse) Validate() error {
	if r.Status < 100 || r.Status > 599 {
		return fmt.Errorf("status code %v is not valid", r.Status)
	}
	return nil
}
