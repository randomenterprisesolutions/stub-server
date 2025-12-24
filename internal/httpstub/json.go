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
}

var _ Stub = &JSONStub{}

// Matches checks if the JSONStub matches the given HTTP request.
func (s JSONStub) Matches(req *http.Request) bool {
	if s.ExactPath != "" {
		return req.URL.Path == s.ExactPath && req.Method == s.HTTPMethod
	}

	match, _ := regexp.MatchString(s.RegexPath, req.URL.Path)
	return match
}

// Write writes the JSONStub response to the provided http.ResponseWriter.
func (s JSONStub) Write(req *http.Request, w http.ResponseWriter) error {
	return s.Response.Write(req, w)
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
		if _, err := regexp.Compile(s.RegexPath); err != nil {
			return fmt.Errorf("invalid regex: %w", err)
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
func (r JSONResponse) Write(_ *http.Request, w http.ResponseWriter) error {
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
