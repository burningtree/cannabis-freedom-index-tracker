package api

import (
	"errors"
	"mime"
	"net"
	"net/http"
	"net/url"

	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/store"
)

func loopbackHost(host string) bool {
	return host == "localhost" || (net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback())
}
func (s *Server) localControls() bool {
	host, _, err := net.SplitHostPort(s.Config.Addr)
	return s.Config.DevControls && err == nil && loopbackHost(host)
}

// Local development intake can bypass rate limits, including through Astro's
// loopback proxy. A client-supplied forwarding header cannot grant this access.
func (s *Server) localDevelopmentRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if !s.localControls() || err != nil || !loopbackHost(host) {
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || !loopbackHost(u.Hostname()) {
			return false
		}
	}
	return true
}

func (s *Server) restart(w http.ResponseWriter, r *http.Request) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if !s.localControls() || err != nil || !loopbackHost(host) {
		problem(w, 403, "Restart is available only on the local development backend.")
		return
	}
	// Require a non-simple JSON request and a local browser origin, including when
	// Astro proxies the request. Never use forwarded IP headers for this control.
	content, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if content != "application/json" || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		problem(w, 403, "Use the localhost submission restart control.")
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, e := url.Parse(origin)
		if e != nil || !loopbackHost(u.Hostname()) {
			problem(w, 403, "Restart requires a localhost origin.")
			return
		}
	}
	id := r.PathValue("id")
	if !idPattern.MatchString(id) {
		problem(w, 404, "Receipt not found.")
		return
	}
	if err = s.Store.Restart(r.Context(), id); err != nil {
		switch {
		case store.IsMissingAI(err):
			problem(w, 404, "Receipt not found.")
		case errors.Is(err, store.ErrRestartConflict):
			problem(w, 409, "This submission is already running or is not failed or budget paused.")
		default:
			problem(w, 503, "Submission could not be restarted.")
		}
		return
	}
	write(w, 202, map[string]string{"id": id, "status": "queued"})
}
