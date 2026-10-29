package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestUnlimitedReservationsKeepUsageAndFiniteLimits(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	id := accept(t, s, false)
	v, _ := s.Get(ctx, id)
	for i := 0; i < 3; i++ {
		if _, err := s.Reserve(ctx, v.CaseID, "private", 100, 0); err != nil {
			t.Fatal(err)
		}
	}
	var total, n int
	s.DB.QueryRow("SELECT sum(reserved_cents),count(*) FROM research_runs").Scan(&total, &n)
	if total != 300 || n != 3 {
		t.Fatal(total, n)
	}
	if _, err := s.Reserve(ctx, v.CaseID, "private", 100, 300); !errors.Is(err, ErrBudget) {
		t.Fatal("finite limit bypassed", err)
	}
	if _, err := s.Reserve(ctx, v.CaseID, "private", 100, -1); err == nil {
		t.Fatal("negative budget accepted")
	}
}
func TestUnlimitedStartupResumesBudgetWaitWithoutLosingUsage(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	id := accept(t, s, false)
	v, _ := s.Get(ctx, id)
	s.Reserve(ctx, v.CaseID, "private", 100, 100)
	j, _ := s.Claim(ctx, "jobs", time.Minute)
	if err := s.Finish(ctx, "jobs", j, ErrBudget); err != nil {
		t.Fatal(err)
	}
	if err := s.ResumeBudgetWait(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.ResumeBudgetWait(ctx); err != nil {
		t.Fatal(err)
	}
	v, _ = s.Get(ctx, id)
	if v.Status != "queued" || v.ResumeAt != 0 || len(v.History) != 3 {
		t.Fatal(v)
	}
	next, err := s.Claim(ctx, "jobs", time.Minute)
	if err != nil || next.ID != j.ID {
		t.Fatal(next, err)
	}
	var used int
	s.DB.QueryRow("SELECT sum(reserved_cents) FROM research_runs").Scan(&used)
	if used != 100 {
		t.Fatal("usage lost", used)
	}
}
func TestBudgetPausedSubmissionCanRestartWithoutBypassingLimit(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	id := accept(t, s, false)
	v, _ := s.Get(ctx, id)
	s.Reserve(ctx, v.CaseID, "private", 100, 100)
	j, _ := s.Claim(ctx, "jobs", time.Minute)
	s.Finish(ctx, "jobs", j, ErrBudget)
	if err := s.Restart(ctx, id); err != nil {
		t.Fatal(err)
	}
	v, _ = s.Get(ctx, id)
	if v.Status != "queued" || v.ResumeAt != 0 {
		t.Fatal(v)
	}
	if _, err := s.Claim(ctx, "jobs", time.Minute); err != nil {
		t.Fatal("restart retained tomorrow delay", err)
	}
	if _, err := s.Reserve(ctx, v.CaseID, "private", 100, 100); !errors.Is(err, ErrBudget) {
		t.Fatal("restart reset budget", err)
	}
	if err := s.Restart(ctx, id); !errors.Is(err, ErrRestartConflict) {
		t.Fatal("duplicate restart accepted", err)
	}
}
