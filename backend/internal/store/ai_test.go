package store

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestAIActivityIsBoundedSharedAndDurable(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	id := accept(t, s, false)
	second := accept(t, s, false)
	v, _ := s.Get(ctx, id)
	run, err := s.Reserve(ctx, v.CaseID, "private-workspace", 100, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.StartRun(ctx, run, "opencode-go/deepseek-v4.1-flash", "local"); err != nil {
		t.Fatal(err)
	}
	before, _ := s.FeedVersion(ctx)
	for i := 0; i < 105; i++ {
		if err = s.AddAIEvent(ctx, v.CaseID, run, "assistant", "message", fmt.Sprintf("Public progress %d", i)); err != nil {
			t.Fatal(err)
		}
	}
	after, _ := s.FeedVersion(ctx)
	if before == after {
		t.Fatal("AI events did not trigger live updates")
	}
	a, err := s.AIActivity(ctx, second, -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Events) != 100 || a.Events[0].Text != "Public progress 5" || a.Events[99].Text != "Public progress 104" || len(a.Runs) != 1 || a.Runs[0].Model != "opencode-go/deepseek-v4.1-flash" {
		t.Fatal("incorrect activity projection")
	}
	if strings.Contains(JSON(a), "private-workspace") {
		t.Fatal("private workspace leaked")
	}
	for i := 0; i < 11; i++ {
		if _, err := s.Reserve(ctx, v.CaseID, "private-retry-workspace", 1, 100000); err != nil {
			t.Fatal(err)
		}
	}
	full, err := s.AIActivityLog(ctx, second)
	if err != nil {
		t.Fatal(err)
	}
	if len(full.Events) != 105 || len(full.Runs) != 12 || full.Events[0].Text != "Public progress 0" || full.Events[104].Text != "Public progress 104" || full.NextCursor != "" {
		t.Fatal("full log was truncated or out of order")
	}
	if strings.Contains(JSON(full), "private-workspace") || strings.Contains(JSON(full), "private-retry-workspace") {
		t.Fatal("private data leaked in export")
	}
	if err = s.EndRun(ctx, run, "completed", "new_event"); err != nil {
		t.Fatal(err)
	}
}
