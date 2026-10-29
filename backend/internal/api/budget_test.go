package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/store"
)

func TestBudgetInfoMatchesReservationsAndUTCDay(t *testing.T) {
	s, _ := server(t)
	s.Config.RunCents = 100
	s.Config.DailyCents = 350
	ctx := context.Background()
	id, err := s.Store.Accept(ctx, store.Intake{Origin: "web", URL: "https://news.example/budget-display", CanonicalURL: "https://news.example/budget-display"})
	if err != nil {
		t.Fatal(err)
	}
	submission, _ := s.Store.Get(ctx, id)
	// Local midnight has passed, but the UTC budget day has not rolled over yet.
	now := time.Date(2026, 10, 30, 0, 30, 0, 0, time.FixedZone("CET", 3600))
	start := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
	for i, status := range []string{"failed", "completed", "failed"} {
		run, err := s.Store.Reserve(ctx, submission.CaseID, "private-workspace", 100, 10000)
		if err != nil {
			t.Fatal(err)
		}
		created := start.Unix() + int64(i)
		if i == 2 {
			created = start.Unix() - 1
		}
		if _, err = s.Store.DB.Exec("UPDATE research_runs SET status=?,created_at=? WHERE id=?", status, created, run); err != nil {
			t.Fatal(err)
		}
	}
	b := s.budgetInfo(ctx, now)
	if b == nil || b.ReservedCents != 200 || b.DailyLimitCents != 350 || b.PerRunCents != 100 || b.RemainingCents != 150 || b.RunsRemaining != 1 || b.ResetsAt != start.Add(24*time.Hour).Unix() {
		t.Fatal(b)
	}
	// The next UTC day is a fresh budget, even though failed attempts still exist.
	next := s.budgetInfo(ctx, start.Add(24*time.Hour))
	if next.ReservedCents != 0 || next.RunsRemaining != 3 {
		t.Fatal(next)
	}
	s.Config.DailyCents = 150
	b = s.budgetInfo(ctx, now)
	if b.ReservedCents != 200 || b.RemainingCents != 0 || b.RunsRemaining != 0 {
		t.Fatal("negative remaining budget", b)
	}
}
func TestSubmissionFeedExposesCurrentBudgetConfiguration(t *testing.T) {
	s, _ := server(t)
	s.Config.RunCents = 75
	s.Config.DailyCents = 600
	response := request(s.Handler(), "GET", "/v1/submissions", "", "")
	var payload struct {
		Processing ProcessingInfo `json:"processing"`
	}
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &payload) != nil {
		t.Fatal(response.Code, response.Body.String())
	}
	b := payload.Processing.Budget
	if b == nil || b.PerRunCents != 75 || b.DailyLimitCents != 600 || b.RunsRemaining != 8 || b.ReservedCents != 0 {
		t.Fatal(b)
	}
}

func TestUnlimitedBudgetInfoStillReportsUsage(t *testing.T) {
	s, _ := server(t)
	s.Config.DailyCents = 0
	s.Config.RunCents = 100
	ctx := context.Background()
	id, err := s.Store.Accept(ctx, store.Intake{Origin: "web", URL: "https://news.example/unlimited", CanonicalURL: "https://news.example/unlimited"})
	if err != nil {
		t.Fatal(err)
	}
	v, _ := s.Store.Get(ctx, id)
	s.Store.Reserve(ctx, v.CaseID, "private", 100, 0)
	b := s.budgetInfo(ctx, time.Now())
	if b == nil || !b.Unlimited || b.DailyLimitCents != 0 || b.ReservedCents != 100 || b.RemainingCents != -1 || b.RunsRemaining != -1 {
		t.Fatal(b)
	}
}
