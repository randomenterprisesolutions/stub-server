package httpstub

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// HTTPStub represents a predefined HTTP stub.
type HTTPStub struct {
	Path         string
	HTTPMethod   string
	ResponsePath string
}

var _ Stub = &HTTPStub{}

// Matches checks if the HTTPStub matches the given HTTP request.
func (s *HTTPStub) Matches(inv HTTPInvocation) bool {
	return inv.Path == s.Path && (s.HTTPMethod == "*" || inv.Method == s.HTTPMethod)
}

// Type returns the MatchType
func (s HTTPStub) Type() MatchType {
	return MatchExact
}

// Invoke writes the HTTPStub response to the provided http.ResponseWriter.
func (s *HTTPStub) Invoke(w http.ResponseWriter) {
	f, err := os.Open(s.ResponsePath)
	if err != nil {
		http.Error(w, "Failed to read response", http.StatusInternalServerError)
		return
	}
	defer func() {
		_ = f.Close()
	}()

	req := &http.Request{
		Method: s.HTTPMethod,
	}
	resp, err := http.ReadResponse(bufio.NewReader(f), req)
	if err != nil {
		http.Error(w, "Failed to write response", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close() //nolint:errcheck

	for k, val := range resp.Header {
		for _, v := range val {
			w.Header().Set(k, v)
		}
	}

	w.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(w, resp.Body); err != nil {
		http.Error(w, "Failed to write response", http.StatusInternalServerError)
		return
	}
}

// Validate validates the HTTPStub fields.
func (s *HTTPStub) Validate() error {
	if s.Path == "" {
		return errors.New(`"path" field is required`)
	}

	if s.HTTPMethod == "" {
		return errors.New(`"method" field is required`)
	}

	if s.ResponsePath == "" {
		return errors.New(`"response_path" field is required`)
	}

	return nil
}

func loadHTTPFile(root string, path string) (s Stub, err error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("open file: %v: %w", path, err)
	}

	relPath, err := filepath.Rel(root, path)
	if err != nil {
		return nil, fmt.Errorf("determine relative path: %w", err)
	}
	relPath = filepath.ToSlash(filepath.Clean(relPath))
	dir, _ := filepath.Split(relPath)
	dir = filepath.Clean(dir)

	// Determine URL and HTTP method from directory structure
	method := filepath.Base(dir)
	if method == "" {
		return nil, fmt.Errorf("could not determine HTTP method from file name: %v", path)
	}

	stub := HTTPStub{
		Path:         "/" + filepath.Dir(dir),
		HTTPMethod:   method,
		ResponsePath: path,
	}

	if err := stub.Validate(); err != nil {
		return nil, fmt.Errorf("stub validation %v: %w", path, err)
	}

	return &stub, nil
}
