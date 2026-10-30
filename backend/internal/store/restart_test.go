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

func TestRetryKeepsResearchAndRestartDropsIt(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	id := accept(t, s, false)
	v, _ := s.Get(ctx, id)
	job, err := s.Claim(ctx, "jobs", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	s.Finish(ctx, "jobs", job, nil)
	s.Transition(ctx, v.CaseID, "failed", "Build failed.", "", "")
	if err = s.Retry(ctx, id); !errors.Is(err, ErrNothingToResume) {
		t.Fatal("retry without research accepted", err)
	}
	s.DB.Exec("INSERT INTO research_checkpoints(case_id,payload,created_at) VALUES(?,?,?)", v.CaseID, "{}", 1)
	s.SetCaseOutput(ctx, v.CaseID, map[string]string{"ai_result": `{"outcome":"new_event"}`, "draft_path": "reports/x.md"})
	if v, _ = s.Get(ctx, id); !v.Resumable {
		t.Fatal("saved research not reported as resumable")
	}
	if err = s.Retry(ctx, id); err != nil {
		t.Fatal(err)
	}
	var saved int
	s.DB.QueryRow("SELECT count(*) FROM research_checkpoints WHERE case_id=?", v.CaseID).Scan(&saved)
	if v, _ = s.Get(ctx, id); saved != 1 || v.Status != "queued" || len(v.Result) == 0 || v.DraftPath != "" {
		t.Fatal("retry lost the research or kept the draft", saved, v.Status, string(v.Result), v.DraftPath)
	}
	job, _ = s.Claim(ctx, "jobs", time.Minute)
	s.Finish(ctx, "jobs", job, nil)
	s.Transition(ctx, v.CaseID, "failed", "Build failed again.", "", "")
	if err = s.Restart(ctx, id); err != nil {
		t.Fatal(err)
	}
	s.DB.QueryRow("SELECT count(*) FROM research_checkpoints WHERE case_id=?", v.CaseID).Scan(&saved)
	if v, _ = s.Get(ctx, id); saved != 0 || v.Resumable || len(v.Result) != 0 {
		t.Fatal("restart kept the research")
	}
}
