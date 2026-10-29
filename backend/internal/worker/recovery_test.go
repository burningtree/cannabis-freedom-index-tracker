package worker

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/config"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/research"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/store"
)

func TestRecoveryUsesCompletedAIWithoutAnotherReservationOrModel(t *testing.T) {
	repo, _ := commitFixture(t)
	os.MkdirAll(filepath.Join(repo, "src/data"), 0755)
	os.WriteFile(filepath.Join(repo, "src/data/countries.json"), []byte(`[]`), 0644)
	s, err := store.Open(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	ctx := context.Background()
	id, err := s.Accept(ctx, store.Intake{Origin: "web", URL: "https://news.example/event", CanonicalURL: "https://news.example/event"})
	if err != nil {
		t.Fatal(err)
	}
	v, _ := s.Get(ctx, id)
	s.Transition(ctx, v.CaseID, "drafted", "Interrupted after research completed.", "", "")
	saved := researchCheckpoint{Result: research.Result{Outcome: "insufficient_evidence", Explanation: "The opened sources do not establish an event."}, Model: "original/model#variant", Dir: "missing-old-checkout"}
	if _, err = s.DB.Exec("INSERT INTO research_checkpoints VALUES(?,?,?)", v.CaseID, store.JSON(saved), time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	s.Recover(ctx)
	j, err := s.Claim(ctx, "jobs", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	// No model, auth or budget configured. A fresh model call would fail this test.
	w := Worker{Store: s, Config: config.Config{RepoPath: repo, Workspaces: t.TempDir()}}
	if err = w.process(ctx, j); err != nil {
		t.Fatal(err)
	}
	v, _ = s.Get(ctx, id)
	if v.Status != "insufficient_evidence" {
		t.Fatal(v)
	}
	var n int
	s.DB.QueryRow("SELECT count(*) FROM research_runs").Scan(&n)
	if n != 0 {
		t.Fatal("made a second AI reservation")
	}
}

func TestRecoveryAfterGitCommitBeforeDatabaseAcknowledgement(t *testing.T) {
	repo, d := commitFixture(t)
	ctx := context.Background()
	s, err := store.Open(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	id, err := s.Accept(ctx, store.Intake{Origin: "web", URL: "https://news.example/event", CanonicalURL: "https://news.example/event"})
	if err != nil {
		t.Fatal(err)
	}
	v, _ := s.Get(ctx, id)
	researchJob, _ := s.Claim(ctx, "jobs", time.Minute)
	s.Finish(ctx, "jobs", researchJob, nil)
	tx, _ := s.DB.Begin()
	store.Enqueue(tx, "jobs", "publish", v.CaseID, store.JSON(d), time.Now().Unix())
	tx.Commit()
	old, _ := s.Claim(ctx, "jobs", time.Hour)
	s.Transition(ctx, v.CaseID, "publishing", "Committing the validated draft.", d.Slug, "")
	sha, err := commitReport(ctx, repo, v.CaseID, d)
	if err != nil {
		t.Fatal(err)
	}
	// Crash window: Git committed, but local_publications/case status not yet saved.
	if err = s.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	j, err := s.Claim(ctx, "jobs", time.Minute)
	if err != nil || j.Kind != "publish" {
		t.Fatal(j, err)
	}
	w := Worker{Store: s, Config: config.Config{RepoPath: repo, Workspaces: filepath.Dir(d.Dir)}}
	if err = w.process(ctx, j); err != nil {
		t.Fatal(err)
	}
	if err = s.Finish(ctx, "jobs", old, nil); err == nil {
		t.Fatal("old worker was not fenced")
	}
	v, _ = s.Get(ctx, id)
	if v.Status != "committed" {
		t.Fatal(v)
	}
	if got := testGit(t, repo, "rev-parse", "HEAD"); got != sha {
		t.Fatal("repeated commit", got, sha)
	}
}
