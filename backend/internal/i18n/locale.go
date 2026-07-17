// Package i18n holds locale parsing shared by all handlers.
package i18n

import "net/http"

const Default = "uz-Latn"

var Supported = map[string]bool{"uz-Latn": true, "uz-Cyrl": true, "ru": true, "kaa": true}

// Parse reads ?locale=; empty → Default; unknown → ok=false.
func Parse(r *http.Request) (string, bool) {
	v := r.URL.Query().Get("locale")
	if v == "" {
		return Default, true
	}
	if !Supported[v] {
		return "", false
	}
	return v, true
}
