package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
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
	page, err := s.ListSubmissions(ctx, "", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 57 || len(page.Submissions) != 50 || page.NextCursor == "" || page.Submissions[0].ID != ids[56] {
		t.Fatal("incorrect page", page)
	}
	if _, err = ParseSubmissionCursor(page.NextCursor); err != nil {
		t.Fatal(err)
	}
	// A new arrival cannot shift an older cursor page or hide same-second receipts.
	if _, err = s.Accept(ctx, Intake{Origin: "web", URL: "https://news.example/new", CanonicalURL: "https://news.example/new"}); err != nil {
		t.Fatal(err)
	}
	older, err := s.ListSubmissions(ctx, page.NextCursor, "", "", "", "")
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
	rss, err := s.ListSubmissions(ctx, "", "queued", "rss", "", "")
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
	if _, err = s.ListSubmissions(ctx, "", "bad", "", "", ""); err == nil {
		t.Fatal("invalid filter accepted")
	}
}
func TestFeedRevisionCoversSharedCasesAndNostrLinks(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	before, _ := s.FeedVersion(ctx)
	id := accept(t, s, false)
	other := legacyReceipt(t, s, id)
	after, _ := s.FeedVersion(ctx)
	if before == after {
		t.Fatal("intake did not invalidate register")
	}
	v, _ := s.Get(ctx, id)
	if err := s.Transition(ctx, v.CaseID, "researching", "Research in progress.", "", ""); err != nil {
		t.Fatal(err)
	}
	page, err := s.ListSubmissions(ctx, "", "researching", "", "", "")
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

func TestSubmissionFeedIncludesCumulativeUsageAndBudgetResume(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	id := accept(t, s, false)
	other := legacyReceipt(t, s, id)
	v, _ := s.Get(ctx, id)
	for i := 0; i < 2; i++ {
		run, err := s.Reserve(ctx, v.CaseID, "private-workspace", 1, 1000)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.DB.Exec("UPDATE research_runs SET input_tokens=?,output_tokens=? WHERE id=?", 100+i, 20+i, run); err != nil {
			t.Fatal(err)
		}
	}
	job, err := s.Claim(ctx, "jobs", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Transition(ctx, v.CaseID, "classifying", "The source is checked.", "", ""); err != nil {
		t.Fatal(err)
	}
	if err = s.Finish(ctx, "jobs", job, ErrBudget); err != nil {
		t.Fatal(err)
	}
	page, err := s.ListSubmissions(ctx, "", "budget_wait", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Submissions) != 2 {
		t.Fatal(page)
	}
	for _, entry := range page.Submissions {
		if entry.InputTokens != 201 || entry.OutputTokens != 41 || entry.RunCount != 2 || entry.Status != "budget_wait" || entry.ResumeAt <= time.Now().Unix() || entry.LastStage != "classifying" {
			t.Fatal(entry)
		}
	}
	detail, err := s.Get(ctx, other)
	if err != nil || detail.Status != "budget_wait" || detail.ResumeAt != page.Submissions[0].ResumeAt || !strings.Contains(detail.Explanation, "No new AI run") {
		t.Fatal(detail, err)
	}
}

func TestSubmissionFeedOrdersByLatestUpdate(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	older, err := s.Accept(ctx, Intake{Origin: "web", URL: "https://news.example/older", CanonicalURL: "https://news.example/older"})
	if err != nil {
		t.Fatal(err)
	}
	newer, err := s.Accept(ctx, Intake{Origin: "web", URL: "https://news.example/newer", CanonicalURL: "https://news.example/newer"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.Get(ctx, older)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec("UPDATE cases SET updated_at=? WHERE id=?", time.Now().Unix()+60, v.CaseID); err != nil {
		t.Fatal(err)
	}
	page, err := s.ListSubmissions(ctx, "", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Submissions) != 2 || page.Submissions[0].ID != older || page.Submissions[1].ID != newer {
		t.Fatal(page.Submissions)
	}
	if page.Submissions[0].UpdatedAt <= page.Submissions[1].UpdatedAt {
		t.Fatal(page.Submissions)
	}
	received, err := s.ListSubmissions(ctx, "", "", "", "received", "asc")
	if err != nil {
		t.Fatal(err)
	}
	if received.Submissions[0].ID != older || received.Submissions[1].ID != newer {
		t.Fatal(received.Submissions)
	}
	if _, err = s.ListSubmissions(ctx, FormatSubmissionCursor("received", "asc", "1", 1), "", "", "source", "asc"); err == nil {
		t.Fatal("cursor from another sort was accepted")
	}
	bySource, err := s.ListSubmissions(ctx, "", "", "", "source", "asc")
	if err != nil || bySource.Submissions[0].URL > bySource.Submissions[1].URL {
		t.Fatal(bySource.Submissions, err)
	}
}
