package api

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLocalDevelopmentIntakeHasNoHourlyIPOrHostLimit(t *testing.T) {
	s, _ := server(t)
	s.Config.DevControls = true
	s.Config.Addr = "127.0.0.1:8080"
	s.Config.Origins = []string{"http://localhost:4322"}
	h := s.Handler()
	for i := 0; i < 105; i++ {
		r := httptest.NewRequest("POST", "/v1/submissions", strings.NewReader(fmt.Sprintf(`{"url":"https://news.example/local-%d"}`, i)))
		r.RemoteAddr = "127.0.0.1:1234"
		if i%2 == 0 {
			r.Header.Set("Origin", "http://localhost:4322")
		} // direct browser and proxied intake
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 202 {
			t.Fatalf("request %d: %d %s", i, w.Code, w.Body.String())
		}
	}
	if len(s.Limit.entries) != 0 {
		t.Fatal("local requests consumed public rate limits")
	}
	r := httptest.NewRequest("POST", "/v1/submissions", strings.NewReader(`{"url":"http://127.0.0.1/private"}`))
	r.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 422 {
		t.Fatal("local mode bypassed URL validation", w.Code)
	}
}
func TestLocalRateExemptionCannotBeGrantedByForwardedHeaders(t *testing.T) {
	for _, test := range []struct {
		name, bind, remote, origin string
		dev                        bool
	}{
		{"public-peer", "127.0.0.1:8080", "198.51.100.20:1234", "", true},
		{"public-bind", "0.0.0.0:8080", "127.0.0.1:1234", "", true},
		{"disabled", "127.0.0.1:8080", "127.0.0.1:1234", "", false},
		{"public-origin", "127.0.0.1:8080", "127.0.0.1:1234", "https://tracker.example", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, _ := server(t)
			s.Config.DevControls = test.dev
			s.Config.Addr = test.bind
			r := httptest.NewRequest("POST", "/v1/submissions", strings.NewReader(`{"url":"https://news.example/report"}`))
			r.RemoteAddr = test.remote
			r.Header.Set("Origin", test.origin)
			r.Header.Set("X-Forwarded-For", "127.0.0.1")
			s.Limit.entries = map[string]bucket{"ip:" + s.clientIP(r): {N: 20, Until: time.Now().Add(time.Hour)}}
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, r)
			if w.Code != 429 {
				t.Fatal(w.Code, w.Body.String())
			}
		})
	}
}
func TestPublicNewsHostLimitRemains(t *testing.T) {
	s, h := server(t)
	s.Limit.entries = map[string]bucket{"host:news.example": {N: 100, Until: time.Now().Add(time.Hour)}}
	w := request(h, "POST", "/v1/submissions", `{"url":"https://news.example/report"}`, "")
	if w.Code != 429 || !strings.Contains(w.Body.String(), "news host") {
		t.Fatal(w.Code, w.Body.String())
	}
}
