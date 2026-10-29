package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.DB.Close() })
	return s
}
func accept(t *testing.T, s *Store, gateway bool) string {
	t.Helper()
	id, err := s.Accept(context.Background(), Intake{Origin: "web", URL: "https://news.example/article", CanonicalURL: "https://news.example/article", Gateway: gateway})
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func TestReceiptsShareCaseAndSurviveRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.sqlite")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	a := accept(t, s, false)
	b := accept(t, s, false)
	if a == b {
		t.Fatal("receipt identity reused")
	}
	av, _ := s.Get(ctx, a)
	bv, _ := s.Get(ctx, b)
	if av.CaseID != bv.CaseID {
		t.Fatal("not deduplicated")
	}
	var count int
	s.DB.QueryRow("SELECT count(*) FROM jobs").Scan(&count)
	if count != 1 {
		t.Fatal(count)
	}
	s.DB.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	v, err := s.Get(ctx, a)
	if err != nil || v.Status != "queued" || len(v.History) != 1 {
		t.Fatalf("lost receipt: %+v %v", v, err)
	}
	j, err := s.Claim(ctx, "jobs", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if j.Ref != av.CaseID {
		t.Fatal("lost job")
	}
}
func TestLeaseRecoveryAndFencing(t *testing.T) {
	s := openTest(t)
	accept(t, s, false)
	ctx := context.Background()
	first, err := s.Claim(ctx, "jobs", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Claim(ctx, "jobs", time.Minute); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("double claim")
	}
	s.DB.Exec("UPDATE jobs SET lease_until=0")
	next, err := s.Claim(ctx, "jobs", time.Minute)
	if err != nil || next.Token == first.Token {
		t.Fatal("lease not recovered")
	}
	if err = s.Finish(ctx, "jobs", first, nil); err == nil {
		t.Fatal("stale worker completed job")
	}
	if err = s.Finish(ctx, "jobs", next, nil); err != nil {
		t.Fatal(err)
	}
}
func TestResolutionTransactionalAndIdempotent(t *testing.T) {
	s := openTest(t)
	id := accept(t, s, true)
	v, _ := s.Get(context.Background(), id)
	for range 2 {
		if err := s.Transition(context.Background(), v.CaseID, "not_relevant", "Not an enforcement event.", "", ""); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	s.DB.QueryRow("SELECT count(*) FROM outbox WHERE kind='resolution'").Scan(&count)
	if count != 1 {
		t.Fatal(count)
	}
	s.DB.QueryRow("SELECT count(*) FROM submission_history WHERE status='not_relevant'").Scan(&count)
	if count != 1 {
		t.Fatal(count)
	}
	second := accept(t, s, true)
	other, _ := s.Get(context.Background(), second)
	if other.Status != "not_relevant" {
		t.Fatal(other)
	}
	s.DB.QueryRow("SELECT count(*) FROM outbox WHERE kind='resolution'").Scan(&count)
	if count != 2 {
		t.Fatal("new receipt lost its resolution")
	}
}
func TestIngestIdempotencyAndBudget(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	in := Intake{Origin: "rss", URL: "https://news.example/x", CanonicalURL: "https://news.example/x", IdempotencyKey: "rss:article"}
	a, err := s.Accept(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Accept(ctx, in)
	if err != nil || a != b {
		t.Fatal("machine ingest not idempotent")
	}
	v, _ := s.Get(ctx, a)
	if _, err = s.Reserve(ctx, v.CaseID, "private", 100, 100); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Reserve(ctx, v.CaseID, "private", 100, 100); err == nil {
		t.Fatal("budget exceeded")
	}
	var count int
	s.DB.QueryRow("SELECT count(*) FROM outbox").Scan(&count)
	if count != 0 {
		t.Fatal("machine candidate posted to Nostr")
	}
}
func TestBackupRestore(t *testing.T) {
	s := openTest(t)
	id := accept(t, s, false)
	backup := filepath.Join(t.TempDir(), "restore.sqlite")
	if err := s.Backup(context.Background(), backup); err != nil {
		t.Fatal(err)
	}
	restored, err := Open(backup)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.DB.Close()
	if _, err = restored.Get(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if err = s.Backup(context.Background(), backup); err == nil {
		t.Fatal("backup overwritten")
	}
}
func TestBudgetWaitDoesNotExhaustAttempts(t *testing.T) {
	s := openTest(t)
	accept(t, s, false)
	j, err := s.Claim(context.Background(), "jobs", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Finish(context.Background(), "jobs", j, ErrBudget); err != nil {
		t.Fatal(err)
	}
	var attempts int
	var status string
	var available int64
	if err = s.DB.QueryRow("SELECT attempts,status,available_at FROM jobs WHERE id=?", j.ID).Scan(&attempts, &status, &available); err != nil {
		t.Fatal(err)
	}
	if attempts != 0 || status != "queued" || available <= time.Now().Unix() {
		t.Fatal(attempts, status, available)
	}
}
func TestLaterHumanOutcomeHasSeparateResolution(t *testing.T) {
	s := openTest(t)
	id := accept(t, s, true)
	v, _ := s.Get(context.Background(), id)
	for _, status := range []string{"needs_review", "not_relevant"} {
		if err := s.Transition(context.Background(), v.CaseID, status, "A reviewer resolved the submission.", "", ""); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	s.DB.QueryRow("SELECT count(*) FROM outbox WHERE kind='resolution'").Scan(&n)
	if n != 2 {
		t.Fatal("later resolution was suppressed", n)
	}
}
