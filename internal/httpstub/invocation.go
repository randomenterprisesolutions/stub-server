package httpstub

import (
	"net/http"
	"net/url"
)

// HTTPInvocation captures request identity for matching.
type HTTPInvocation struct {
	Method  string
	Path    string
	Query   url.Values
	Headers http.Header
}
