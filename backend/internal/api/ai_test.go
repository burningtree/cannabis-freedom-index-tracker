package api

import (
	"context"
	"encoding/json"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/research"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/store"
	"strings"
	"testing"
)

func TestAIEndpointAndMissingConfigurationAreExplicit(t *testing.T) {
	s, _ := server(t)
	s.Config.AutoPublish = true
	s.Health = research.NewHealth()
	s.Config.ResearchRunner = "local"
	s.Config.ResearchModel = "opencode-go/deepseek-v4.1-flash"
	h := s.Handler()
	id, err := s.Store.Accept(context.Background(), store.Intake{Origin: "web", URL: "https://news.example/a", CanonicalURL: "https://news.example/a"})
	if err != nil {
		t.Fatal(err)
	}
	w := request(h, "GET", "/v1/submissions/"+id+"/ai", "", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Checking the OpenCode binary") || !strings.Contains(w.Body.String(), "deepseek-v4.1-flash") {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, path := range []string{"/v1/submissions/bad/ai", "/v1/submissions/" + strings.Repeat("a", 32) + "/ai"} {
		if r := request(h, "GET", path, "", ""); r.Code != 404 {
			t.Fatal(r.Code)
		}
	}
	if r := request(h, "GET", "/v1/submissions/"+id+"/ai?after=bad", "", ""); r.Code != 400 {
		t.Fatal(r.Code)
	}
}

func TestFullAIJSONLogEndpoint(t *testing.T) {
	s, _ := server(t)
	id, err := s.Store.Accept(context.Background(), store.Intake{Origin: "web", URL: "https://news.example/json", CanonicalURL: "https://news.example/json"})
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler()
	response := request(h, "GET", "/v1/submissions/"+id+"/ai/log", "", "")
	if response.Code != 200 || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(response.Code, response.Body.String())
	}
	var payload struct {
		SchemaVersion int              `json:"schema_version"`
		SubmissionID  string           `json:"submission_id"`
		Activity      store.AIActivity `json:"activity"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.SchemaVersion != 1 || payload.SubmissionID != id || payload.Activity.Events == nil || payload.Activity.Runs == nil {
		t.Fatal("invalid JSON export")
	}
	for _, id := range []string{"bad", strings.Repeat("a", 32)} {
		if response := request(h, "GET", "/v1/submissions/"+id+"/ai/log", "", ""); response.Code != 404 {
			t.Fatal(response.Code)
		}
	}
}
