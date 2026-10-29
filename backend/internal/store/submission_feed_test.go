package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func TestSubmissionFeedPagesEveryOriginWithoutPrivateData(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	origins := []string{"web", "nostr", "rss", "gdelt", "api"}
	ids := []string{}
	for i := 0; i < 57; i++ {
		id, err := s.Accept(ctx, Intake{Origin: origins[i%len(origins)], URL: fmt.Sprintf("https://news.example/%d", i), CanonicalURL: fmt.Sprintf("https://news.example/%d", i), Note: "A public note", Provenance: "PRIVATE: research provenance"})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	page, err := s.ListSubmissions(ctx, 1<<62, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 57 || len(page.Submissions) != 50 || page.NextCursor == "" || page.Submissions[0].ID != ids[56] {
		t.Fatal("incorrect page", page)
	}
	cursor, err := strconv.ParseInt(page.NextCursor, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	// A new arrival cannot shift an older cursor page or hide same-second receipts.
	if _, err = s.Accept(ctx, Intake{Origin: "web", URL: "https://news.example/new", CanonicalURL: "https://news.example/new"}); err != nil {
		t.Fatal(err)
	}
	older, err := s.ListSubmissions(ctx, cursor, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(older.Submissions) != 7 || older.NextCursor != "" || older.Total != 58 || older.Submissions[6].ID != ids[0] {
		t.Fatal("unstable pagination", older)
	}
	seen := map[string]bool{}
	for _, v := range append(page.Submissions, older.Submissions...) {
		if seen[v.ID] {
			t.Fatal("duplicate page receipt")
		}
		seen[v.ID] = true
	}
	if len(seen) != 57 {
		t.Fatal("missing receipts")
	}
	b, _ := json.Marshal(page)
	if strings.Contains(string(b), "PRIVATE") {
		t.Fatal("private provenance leaked")
	}
	rss, err := s.ListSubmissions(ctx, 1<<62, "queued", "rss")
	if err != nil {
		t.Fatal(err)
	}
	if rss.Total != 11 {
		t.Fatal(rss.Total)
	}
	for _, v := range rss.Submissions {
		if v.Origin != "rss" || v.Status != "queued" {
			t.Fatal(v)
		}
	}
	if _, err = s.ListSubmissions(ctx, 1<<62, "bad", ""); err == nil {
		t.Fatal("invalid filter accepted")
	}
}
func TestFeedRevisionCoversSharedCasesAndNostrLinks(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	before, _ := s.FeedVersion(ctx)
	id := accept(t, s, false)
	other := accept(t, s, false)
	after, _ := s.FeedVersion(ctx)
	if before == after {
		t.Fatal("intake did not invalidate register")
	}
	v, _ := s.Get(ctx, id)
	if err := s.Transition(ctx, v.CaseID, "researching", "Research in progress.", "", ""); err != nil {
		t.Fatal(err)
	}
	page, err := s.ListSubmissions(ctx, 1<<62, "researching", "")
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || page.Revision == after {
		t.Fatal("case change missing from feed")
	}
	if page.Submissions[0].ID != other {
		t.Fatal("status change reordered receipt")
	}
	previous := page.Revision
	if _, err = s.DB.Exec("UPDATE submissions SET gateway_event_id=? WHERE id=?", strings.Repeat("a", 64), id); err != nil {
		t.Fatal(err)
	}
	next, _ := s.FeedVersion(ctx)
	if next == previous {
		t.Fatal("Nostr update did not invalidate register")
	}
}
