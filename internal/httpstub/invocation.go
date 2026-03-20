package httpstub

import (
	"net/http"
	"net/url"
)

// HTTPInvocation holds information about an HTTP request for stub matching.
type HTTPInvocation struct {
	Method  string
	Path    string
	Query   url.Values
	Headers http.Header
	Body    map[string]any
}
