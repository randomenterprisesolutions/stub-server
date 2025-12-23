package httpstub

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// HTTPStub represents a predefined HTTP stub.
type HTTPStub struct {
	Path       string
	HTTPMethod string
	Response   string
}

var _ Stub = &HTTPStub{}

// URL returns the URL path of the HTTP stub.
func (s HTTPStub) URL() string {
	return s.Path
}

// Method returns the HTTP method of the stub.
func (s HTTPStub) Method() string {
	return s.HTTPMethod
}

// Write writes the HTTPStub response to the provided http.ResponseWriter.
func (s HTTPStub) Write(req *http.Request, w http.ResponseWriter) error {
	resp, err := http.ReadResponse(bufio.NewReader(strings.NewReader(s.Response)), req)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	defer resp.Body.Close()

	for k, val := range resp.Header {
		for _, v := range val {
			w.Header().Set(k, v)
		}
	}

	w.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("copy body: %w", err)
	}

	return nil
}

// Validate validates the HTTPStub fields.
func (s *HTTPStub) Validate() error {
	if s.Path == "" {
		return errors.New(`"path" field is required`)
	}

	if s.HTTPMethod == "" {
		return errors.New(`"method" field is required`)
	}

	return nil
}

func loadHTTPFile(root string, path string) (s Stub, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %v: %w", path, err)
	}
	defer func() {
		closeErr := f.Close()
		if closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close file: %w", closeErr))
		}
	}()

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

	rawResponse, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("read file: %v: %w", path, err)
	}

	stub := HTTPStub{
		Path:       "/" + filepath.Dir(dir),
		HTTPMethod: method,
		Response:   string(rawResponse),
	}

	if err := stub.Validate(); err != nil {
		return nil, fmt.Errorf("stub validation %v: %w", path, err)
	}

	return stub, nil
}
