package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestKeywordMigrationOnlyRetriesUnresearchedKeywordRejections(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "db.sqlite")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for i, explanation := range []string{
		"The submitted page (aktuality.sk, 1,553 characters of text) does not mention cannabis.",
		"The submitted page (jcted.cz, 2,800 characters of text) does not mention cannabis.",
		"The AI determined this article is outside the tracker scope.",
	} {
		url := "https://news.example/" + ID()
		id, err := s.Accept(ctx, Intake{Origin: "web", URL: url, CanonicalURL: url})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
		v, _ := s.Get(ctx, id)
		j, err := s.Claim(ctx, "jobs", time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		s.Transition(ctx, v.CaseID, "not_relevant", explanation, "", "")
		s.Finish(ctx, "jobs", j, nil)
		if i == 1 {
			s.Reserve(ctx, v.CaseID, "old-run", 100, 0)
		} // an AI-reviewed outcome must stay put
	}
	s.DB.Exec("DELETE FROM schema_migrations WHERE name='013_keyword_recovery.sql'")
	s.DB.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	for i, id := range ids {
		v, _ := s.Get(ctx, id)
		want := "not_relevant"
		if i == 0 {
			want = "queued"
		}
		if v.Status != want {
			t.Fatal(i, v.Status, want)
		}
	}
	next, err := s.Claim(ctx, "jobs", time.Minute)
	if err != nil || next.Kind != "research" || next.Attempts != 1 {
		t.Fatal(next, err)
	}
}
