package httpstub

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// JSONStub represents a predefined HTTP stub.
type JSONStub struct {
	Path       string       `json:"path"`
	HTTPMethod string       `json:"method"`
	Response   JSONResponse `json:"response"`
}

var _ Stub = &JSONStub{}

// URL returns the URL path of the JSON stub.
func (s JSONStub) URL() string {
	return s.Path
}

// Method returns the HTTP method of the stub.
func (s JSONStub) Method() string {
	return s.HTTPMethod
}

// Write writes the JSONStub response to the provided http.ResponseWriter.
func (s JSONStub) Write(req *http.Request, w http.ResponseWriter) error {
	return s.Response.Write(req, w)
}

// Validate validates the JSONStub fields.
func (s *JSONStub) Validate() error {
	if s.Path == "" {
		return errors.New(`"path" field is required`)
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
