// Package proxy serves the classroom browser from 127.0.0.1.
//
// Everything the page requests is same-origin, so there is no CORS to
// negotiate and no cookie to steal. API calls are rewritten from the
// frontend's /api/proxy/* path onto the backend's /api/v1/* and signed with
// the station token — which is why the token never reaches JavaScript.
package proxy

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// apiPrefix is the path the Next.js client already calls; keeping it means the
// web app needs no station-specific branch.
const apiPrefix = "/api/proxy/"

// New routes API calls to apiBase with a station token attached and every
// other request to frontendBase untouched.
func New(frontendBase, apiBase string, token func(context.Context) (string, error)) http.Handler {
	frontURL, err := url.Parse(frontendBase)
	if err != nil {
		panic("proxy: bad frontend base: " + err.Error())
	}
	apiURL, err := url.Parse(apiBase)
	if err != nil {
		panic("proxy: bad api base: " + err.Error())
	}

	// NewSingleHostReverseProxy's director rewrites URL.Scheme and URL.Host but
	// deliberately leaves Request.Host alone, so the outgoing request would
	// still announce Host: 127.0.0.1:<agent port>. Both upstreams sit behind
	// Cloudflare, which routes on that header and answers 403 for a host
	// outside the zone -- the classroom would get a Cloudflare error page
	// instead of the kiosk. Wrap the director to name the real upstream.
	frontProxy := httputil.NewSingleHostReverseProxy(frontURL)
	frontDirector := frontProxy.Director
	frontProxy.Director = func(r *http.Request) {
		frontDirector(r)
		r.Host = frontURL.Host
	}

	apiProxy := &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.URL.Scheme = apiURL.Scheme
			r.URL.Host = apiURL.Host
			r.Host = apiURL.Host
			r.URL.Path = "/api/v1/" + strings.TrimPrefix(r.URL.Path, apiPrefix)
		},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, apiPrefix) {
			frontProxy.ServeHTTP(w, r)
			return
		}
		tok, err := token(r.Context())
		if err != nil || tok == "" {
			// Fail closed: serving an unauthenticated API call would silently
			// downgrade the classroom to the free tier mid-lesson.
			http.Error(w, `{"error":{"code":"station_offline","message":"station token unavailable"}}`,
				http.StatusServiceUnavailable)
			return
		}
		r.Header.Set("Authorization", "Bearer "+tok)
		apiProxy.ServeHTTP(w, r)
	})
}
