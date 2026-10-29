package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestMigrationRepairsLegacySourceCheckedBudgetWait(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.sqlite")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	id, err := s.Accept(ctx, Intake{Origin: "web", URL: "https://news.example/budget", CanonicalURL: "https://news.example/budget"})
	if err != nil {
		t.Fatal(err)
	}
	v, _ := s.Get(ctx, id)
	job, _ := s.Claim(ctx, "jobs", time.Minute)
	if err = s.Finish(ctx, "jobs", job, ErrBudget); err != nil {
		t.Fatal(err)
	}
	// Reconstruct the state produced by older backends: queued budget job, stale case label.
	for _, query := range []string{"UPDATE cases SET status='classifying',explanation='Source checked.'", "DELETE FROM submission_history WHERE status='budget_wait'", "DELETE FROM schema_migrations WHERE name='008_budget_wait_and_feed_stats.sql'"} {
		if _, err = s.DB.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	s.DB.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	result, err := s.Get(ctx, id)
	if err != nil || result.Status != "budget_wait" || result.ResumeAt <= time.Now().Unix() {
		t.Fatal(result, err)
	}
	if result.History[len(result.History)-1].Status != "budget_wait" || result.CaseID != v.CaseID {
		t.Fatal("repair did not retain shared case and history")
	}
}
