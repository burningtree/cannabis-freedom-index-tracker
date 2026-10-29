package research

import (
	"context"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/store"
	"path/filepath"
	"strings"
	"testing"
)

// The chat shows the run as it happened (the data is public): reasoning, the address a tool opened and
// the final answer appear; credentials are redacted and a tool's raw output (a page's text) stays out.
func TestPublicEventsShowTheRunButRedactCredentials(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(filepath.Join(t.TempDir(), "events.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	id, err := s.Accept(ctx, store.Intake{Origin: "web", URL: "https://news.example/a", CanonicalURL: "https://news.example/a"})
	if err != nil {
		t.Fatal(err)
	}
	v, _ := s.Get(ctx, id)
	run, err := s.Reserve(ctx, v.CaseID, "", 100, 1000)
	if err != nil {
		t.Fatal(err)
	}
	s.StartRun(ctx, run, "opencode-go/deepseek-v4.1-flash", "local")
	writer := eventWriter{ctx: ctx, store: s, cid: v.CaseID, run: run, secret: "actual-private-test-key"}
	lines := []string{`{"type":"step_start","sessionID":"ses_example"}`, `{"type":"reasoning","part":{"text":"PRIVATE REASONING"}}`, `{"type":"tool_use","part":{"tool":"webfetch","state":{"status":"completed","output":"PRIVATE ARTICLE BODY","input":{"url":"http://private.example/secret"}}}}`, `{"type":"text","part":{"text":"Checking actual-private-test-key and Bearer secretvalue before research."}}`, `{"type":"text","part":{"text":"{\"outcome\":\"new_event\",\"facts\":[\"PRIVATE QUOTATION\"]}"}}`, `{"type":"step_finish","part":{"tokens":{"input":120,"output":35},"cost":0.01}}`}
	for _, line := range lines {
		b := []byte(line + "\n")
		if _, err = writer.Write(b[:len(b)/2]); err != nil {
			t.Fatal(err)
		}
		if _, err = writer.Write(b[len(b)/2:]); err != nil {
			t.Fatal(err)
		}
	}
	activity, err := s.AIActivity(ctx, id, -1)
	if err != nil {
		t.Fatal(err)
	}
	public := store.JSON(activity)
	for _, hidden := range []string{"PRIVATE ARTICLE BODY", "actual-private-test-key", "secretvalue"} {
		if strings.Contains(public, hidden) {
			t.Fatal("should not be in the chat:", hidden)
		}
	}
	for _, shown := range []string{"PRIVATE REASONING", "http://private.example/secret", "PRIVATE QUOTATION", "[redacted]"} {
		if !strings.Contains(public, shown) {
			t.Fatal("missing from the chat:", shown)
		}
	}
	if activity.Runs[0].InputTokens != 120 || activity.Runs[0].OutputTokens != 35 || activity.Runs[0].SessionID != "ses_example" {
		t.Fatal("session/usage missing")
	}
}
func TestFinalArtifactSurvivesPublicProgressMessages(t *testing.T) {
	result := store.JSON(valid())
	stream := store.JSON(map[string]any{"type": "text", "part": map[string]string{"text": "I am checking the published source."}}) + "\n" + store.JSON(map[string]any{"type": "text", "part": map[string]string{"text": result}}) + "\n"
	final, err := FinalText([]byte(stream))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Strict(final); err != nil {
		t.Fatal(err)
	}
}
