package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/store"
)

func TestLocalRestartEndpoint(t *testing.T) {
	s, _ := server(t)
	s.Config.DevControls = true
	s.Config.Addr = "127.0.0.1:8080"
	s.Config.Origins = []string{"http://localhost:4322", "https://tracker.example"}
	id, err := s.Store.Accept(context.Background(), store.Intake{Origin: "web", URL: "https://news.example/restart", CanonicalURL: "https://news.example/restart"})
	if err != nil {
		t.Fatal(err)
	}
	v, _ := s.Store.Get(context.Background(), id)
	job, _ := s.Store.Claim(context.Background(), "jobs", time.Minute)
	s.Store.Finish(context.Background(), "jobs", job, nil)
	s.Store.Transition(context.Background(), v.CaseID, "insufficient_evidence", "An old minor uncertainty blocked this report.", "", "")
	call := func(remote, origin, content string) int {
		r := httptest.NewRequest("POST", "/v1/submissions/"+id+"/restart", strings.NewReader("{}"))
		r.RemoteAddr = remote
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", content)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w.Code
	}
	for _, test := range []struct{ remote, origin, content string }{
		{"203.0.113.1:1234", "http://localhost:4322", "application/json"},
		{"127.0.0.1:1234", "https://tracker.example", "application/json"},
		{"127.0.0.1:1234", "", "text/plain"},
	} {
		if status := call(test.remote, test.origin, test.content); status != 403 {
			t.Fatal(status)
		}
	}
	s.Config.DevControls = false
	if status := call("127.0.0.1:1234", "", "application/json"); status != 403 {
		t.Fatal("development controls enabled by default")
	}
	s.Config.DevControls = true
	s.Config.Addr = "0.0.0.0:8080"
	if status := call("127.0.0.1:1234", "", "application/json"); status != 403 {
		t.Fatal("publicly bound backend allowed restart")
	}
	s.Config.Addr = "127.0.0.1:8080"
	if status := call("127.0.0.1:1234", "http://localhost:4322", "application/json"); status != 202 {
		t.Fatal(status)
	}
	if status := call("127.0.0.1:1234", "", "application/json"); status != 409 {
		t.Fatal("duplicate restart accepted", status)
	}
}
