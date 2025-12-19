package ports

import "net/http"

// HTTPClient abstracts HTTP operations for dependency injection.
// The standard *http.Client satisfies this interface.
type HTTPClient interface {
	// Do sends an HTTP request and returns an HTTP response.
	Do(req *http.Request) (*http.Response, error)
}
