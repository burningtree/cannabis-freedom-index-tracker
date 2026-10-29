package research

import (
	"context"
	"encoding/json"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/store"
	"path/filepath"
	"strings"
	"testing"
)

// A long final answer in Thai (three bytes a character) reaches the chat and never stops the run: a
// failed write of a chat message once made OpenCode's output unreadable at the very last step.
func TestLongAnswerIsStoredAndNeverStopsTheRun(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(filepath.Join(t.TempDir(), "long.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	id, err := s.Accept(ctx, store.Intake{Origin: "web", URL: "https://news.example/th", CanonicalURL: "https://news.example/th"})
	if err != nil {
		t.Fatal(err)
	}
	v, _ := s.Get(ctx, id)
	run, err := s.Reserve(ctx, v.CaseID, "", 100, 1000)
	if err != nil {
		t.Fatal(err)
	}
	writer := eventWriter{ctx: ctx, store: s, cid: v.CaseID, run: run}
	answer, _ := json.Marshal(map[string]any{"outcome": "new_event", "facts": []string{strings.Repeat("ตำรวจตระเวนชายแดนเข้าตรวจสอบพื้นที่ป่า ", 300)}})
	line, _ := json.Marshal(map[string]any{"type": "text", "part": map[string]string{"text": string(answer)}})
	if _, err = writer.Write(append(line, '\n')); err != nil {
		t.Fatal("a chat message stopped the run:", err)
	}
	activity, err := s.AIActivity(ctx, id, -1)
	if err != nil || len(activity.Events) != 1 || activity.Events[0].Kind != "answer" {
		t.Fatalf("answer not stored: %v %+v", err, activity.Events)
	}
}
