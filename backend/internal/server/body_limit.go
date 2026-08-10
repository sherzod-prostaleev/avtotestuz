package server

import (
	"net/http"
	"strings"
)

const defaultMaxRequestBodyBytes int64 = 2 << 20
const supportUploadMaxBodyBytes int64 = 15 << 20

// limitRequestBody bounds every request body before any JSON decoder runs.
// Support chat multipart uploads get a higher ceiling; the upload handler
// still enforces its own MIME/size allowlist.
func limitRequestBody(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body == nil || r.Body == http.NoBody {
				next.ServeHTTP(w, r)
				return
			}
			limit := maxBytes
			if isSupportUploadPath(r.URL.Path) {
				limit = supportUploadMaxBodyBytes
			}
			if r.ContentLength > limit {
				w.Header().Set("Connection", "close")
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}

func isSupportUploadPath(path string) bool {
	return strings.HasSuffix(path, "/support/attachments") ||
		(strings.Contains(path, "/support/conversations/") && strings.HasSuffix(path, "/attachments"))
}
