package server

import (
	"net/http"
)

const defaultMaxRequestBodyBytes int64 = 2 << 20

// limitRequestBody bounds every request body before any JSON decoder runs.
// The API has no multipart upload endpoint; media is served from object
// storage, so a uniform 2 MiB ceiling is intentionally generous.
func limitRequestBody(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body == nil || r.Body == http.NoBody {
				next.ServeHTTP(w, r)
				return
			}
			if r.ContentLength > maxBytes {
				w.Header().Set("Connection", "close")
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
