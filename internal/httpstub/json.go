package httpstub

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
)

// JSONStub represents a predefined HTTP stub.
type JSONStub struct {
	ExactPath  string       `json:"path"`
	RegexPath  string       `json:"regex"`
	HTTPMethod string       `json:"method"`
	Response   JSONResponse `json:"response"`
	regex      *regexp.Regexp
}

var _ Stub = &JSONStub{}

// Matches checks if the JSONStub matches the given HTTP request.
func (s JSONStub) Matches(inv HTTPInvocation) bool {
	if s.ExactPath != "" {
		return inv.Path == s.ExactPath && inv.Method == s.HTTPMethod
	}

	if s.regex != nil {
		return s.regex.MatchString(inv.Path)
	}

	match, _ := regexp.MatchString(s.RegexPath, inv.Path)
	return match
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

	if s.RegexPath != "" {
		compiled, err := regexp.Compile(s.RegexPath)
		if err != nil {
			return fmt.Errorf("invalid regex: %w", err)
		}
		s.regex = compiled
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
