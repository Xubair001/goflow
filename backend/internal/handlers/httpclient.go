package handlers

import "net/http"

// defaultClient returns c, or http.DefaultClient if c is nil. Shared by any
// handler that makes outbound HTTP requests.
func defaultClient(c *http.Client) *http.Client {
	if c != nil {
		return c
	}
	return http.DefaultClient
}

// capOrDefault returns n if positive, otherwise def. Shared by handlers
// that bound how many response bytes they'll read.
func capOrDefault(n, def int64) int64 {
	if n > 0 {
		return n
	}
	return def
}
