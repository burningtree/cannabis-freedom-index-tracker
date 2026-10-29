package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRestartRequeuesSharedFailedCaseAndKeepsHistory(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	id := accept(t, s, false)
	second := accept(t, s, false)
	v, _ := s.Get(ctx, id)
	job, err := s.Claim(ctx, "jobs", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Transition(ctx, v.CaseID, "insufficient_evidence", "Previous incidental open questions.", "", ""); err != nil {
		t.Fatal(err)
	}
	// An active lease must finish before a restart can replace it.
	if err = s.Restart(ctx, id); !errors.Is(err, ErrRestartConflict) {
		t.Fatal(err)
	}
	if err = s.Finish(ctx, "jobs", job, nil); err != nil {
		t.Fatal(err)
	}
	run, err := s.Reserve(ctx, v.CaseID, "old-private-workspace", 1, 1000)
	if err != nil {
		t.Fatal(err)
	}
	s.AddAIEvent(ctx, v.CaseID, run, "assistant", "message", "Earlier run retained.")
	tx, _ := s.DB.BeginTx(ctx, nil)
	Enqueue(tx, "jobs", "publish", v.CaseID, "stale-draft", time.Now().Unix())
	tx.Commit()
	if err = s.Restart(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err = s.Restart(ctx, id); !errors.Is(err, ErrRestartConflict) {
		t.Fatal("double restart accepted", err)
	}
	other, err := s.Get(ctx, second)
	if err != nil || other.Status != "queued" || len(other.History) != 3 {
		t.Fatal(other, err)
	}
	activity, err := s.AIActivityLog(ctx, id)
	if err != nil || len(activity.Runs) != 1 || len(activity.Events) != 1 {
		t.Fatal("lost previous AI activity", err)
	}
	next, err := s.Claim(ctx, "jobs", time.Minute)
	if err != nil || next.Kind != "research" || next.Attempts != 1 {
		t.Fatal(next, err)
	}
	var publications int
	s.DB.QueryRow("SELECT count(*) FROM jobs WHERE kind='publish' AND ref=?", v.CaseID).Scan(&publications)
	if publications != 0 {
		t.Fatal("stale publication survived restart")
	}
}
func TestSuccessfulAndRejectedSubmissionsCannotRestart(t *testing.T) {
	for _, status := range []string{"queued", "researching", "not_relevant", "duplicate", "routed_to_cfi"} {
		t.Run(status, func(t *testing.T) {
			s := openTest(t)
			id := accept(t, s, false)
			v, _ := s.Get(context.Background(), id)
			if status != "queued" {
				s.Transition(context.Background(), v.CaseID, status, "Fixture outcome.", "", "")
			}
			if err := s.Restart(context.Background(), id); !errors.Is(err, ErrRestartConflict) {
				t.Fatal(status, err)
			}
		})
	}
}
