package store

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

func TestNormalizedURLReuseAcrossOriginsAndTerminalStates(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	first, err := s.Accept(ctx, Intake{Origin: "web", URL: "https://News.Example/story?utm_source=one#top", CanonicalURL: "https://News.Example/story?utm_source=one#top", Note: "Original note"})
	if err != nil {
		t.Fatal(err)
	}
	v, _ := s.Get(ctx, first)
	if err = s.Transition(ctx, v.CaseID, "failed", "Could not load source.", "", ""); err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"web", "rss", "gdelt", "nostr"} {
		id, err := s.Accept(ctx, Intake{Origin: origin, URL: "https://news.example/story?utm_source=two", CanonicalURL: "https://news.example/story?utm_source=two", Note: "Different note", Gateway: true, EventID: origin + "-event", IdempotencyKey: origin + ":new-key"})
		if err != nil || id != first {
			t.Fatal(origin, id, err)
		}
	}
	var receipts, history, posts int
	s.DB.QueryRow("SELECT count(*) FROM submissions").Scan(&receipts)
	s.DB.QueryRow("SELECT count(*) FROM submission_history").Scan(&history)
	s.DB.QueryRow("SELECT count(*) FROM outbox").Scan(&posts)
	v, err = s.Get(ctx, first)
	if err != nil || receipts != 1 || history != 2 || posts != 0 || v.Status != "failed" || v.Note != "Original note" {
		t.Fatal(v, receipts, history, posts, err)
	}
	other, err := s.Accept(ctx, Intake{Origin: "web", URL: "https://news.example/another", CanonicalURL: "https://news.example/another"})
	if err != nil || other == first {
		t.Fatal(other, err)
	}
}

func TestConcurrentIntakeCreatesOneReceipt(t *testing.T) {
	s := openTest(t)
	var wg sync.WaitGroup
	ids := make(chan string, 24)
	errs := make(chan error, 24)
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id, err := s.Accept(context.Background(), Intake{Origin: "web", URL: "https://news.example/race", CanonicalURL: "https://news.example/race", IdempotencyKey: fmt.Sprint("request-", i)})
			ids <- id
			errs <- err
		}(i)
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var expected string
	for id := range ids {
		if expected == "" {
			expected = id
		}
		if id != expected {
			t.Fatal("concurrent duplicate", id, expected)
		}
	}
	var count int
	s.DB.QueryRow("SELECT count(*) FROM submissions").Scan(&count)
	if count != 1 {
		t.Fatal(count)
	}
}

func TestHistoricalDuplicateReceiptsSurviveMigrationAndReuseEarliest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.sqlite")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	first := accept(t, s, false)
	var cid string
	s.DB.QueryRow("SELECT case_id FROM submissions WHERE id=?", first).Scan(&cid)
	if _, err = s.DB.Exec("UPDATE submissions SET created_at=1 WHERE id=?", first); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec("INSERT INTO submissions(id,origin,original_url,canonical_url,case_id,created_at) VALUES('legacy-duplicate','web','https://news.example/article','https://news.example/article',?,2)", cid); err != nil {
		t.Fatal(err)
	}
	s.DB.Exec("DROP INDEX submissions_canonical_lookup")
	s.DB.Exec("DELETE FROM schema_migrations WHERE name='014_submission_url_lookup.sql'")
	s.DB.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	id := accept(t, s, false)
	if id != first {
		t.Fatal("oldest receipt not reused", id, first)
	}
	var count int
	s.DB.QueryRow("SELECT count(*) FROM submissions").Scan(&count)
	if count != 2 {
		t.Fatal("historical receipts removed or new duplicate created", count)
	}
}

func TestExistingURLIsReturnedEvenWhenQueueIsFull(t *testing.T) {
	s := openTest(t)
	id := accept(t, s, false)
	_, err := s.DB.Exec(`WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<999)
 INSERT INTO jobs(id,kind,ref,payload,available_at) SELECT printf('capacity-%d',x),'capacity',printf('capacity-%d',x),'{}',0 FROM n`)
	if err != nil {
		t.Fatal(err)
	}
	if next := accept(t, s, false); next != id {
		t.Fatal(next, id)
	}
	if _, err = s.Accept(context.Background(), Intake{Origin: "web", URL: "https://news.example/new", CanonicalURL: "https://news.example/new"}); err == nil {
		t.Fatal("new URL bypassed queue capacity")
	}
}
