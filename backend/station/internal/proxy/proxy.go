// Package proxy serves the classroom browser from 127.0.0.1.
//
// Everything the page requests is same-origin, so there is no CORS to
// negotiate and no cookie to steal. API calls are rewritten from the
// frontend's /api/proxy/* path onto the backend's /api/v1/* and signed with
// the station token — which is why the token never reaches JavaScript.
package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"avtotest.uz/station/internal/netclient"
	"avtotest.uz/station/internal/status"
)

// apiPrefix is the path the Next.js client already calls; keeping it means the
// web app needs no station-specific branch.
const apiPrefix = "/api/proxy/"

// StatusPath is the one route this proxy answers itself.
//
// The agent runs with no console, so the kiosk page in the browser is the only
// screen anyone at the school ever sees. Before this existed, a PC that could
// not get a token showed an endless "Ulanmoqda…" spinner with no version, no
// station id and no error -- identical output for a three-second cold boot and
// for a PC permanently registered to the wrong school. The double underscore
// keeps it clear of any real Next.js route.
const StatusPath = "/__station/status"

// New routes API calls to apiBase with a station token attached and every
// other request to frontendBase untouched.
//
// st may be nil, in which case StatusPath is proxied upstream like any other
// path; every real caller passes one.
func New(frontendBase, apiBase string, token func(context.Context) (string, error), st *status.Store) http.Handler {
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
	transport := netclient.NewTransport()
	frontProxy.Transport = transport
	frontDirector := frontProxy.Director
	frontProxy.Director = func(r *http.Request) {
		frontDirector(r)
		r.Host = frontURL.Host
	}

	apiProxy := &httputil.ReverseProxy{
		Transport: transport,
		Director: func(r *http.Request) {
			r.URL.Scheme = apiURL.Scheme
			r.URL.Host = apiURL.Host
			r.Host = apiURL.Host
			r.URL.Path = "/api/v1/" + strings.TrimPrefix(r.URL.Path, apiPrefix)
		},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if st != nil && r.URL.Path == StatusPath {
			// Answered locally and never cached: this is the one thing that
			// must still work when the backend is unreachable, which is
			// exactly when someone is looking at it.
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			_ = json.NewEncoder(w).Encode(st.Get())
			return
		}
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
