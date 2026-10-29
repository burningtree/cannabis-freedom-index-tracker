package store

import (
	"context"
	"testing"
	"time"
)

func TestAIUsageCountsRunsOnceAndSeparatesPricing(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if _, err := s.Accept(ctx, Intake{Origin: "web", URL: "https://news.example/usage", CanonicalURL: "https://news.example/usage"}); err != nil {
			t.Fatal(err)
		}
	}
	var cid string
	if err := s.DB.QueryRow("SELECT id FROM cases LIMIT 1").Scan(&cid); err != nil {
		t.Fatal(err)
	}
	// Thursday off-peak, peak, reported cost, Friday off-peak, Saturday's
	// same peak hour (off-peak on weekends), then an unstarted reservation.
	times := []string{"2026-10-29T12:00:00Z", "2026-10-29T07:00:00Z", "2026-10-29T12:30:00Z", "2026-10-30T12:00:00Z", "2026-10-31T07:00:00Z", ""}
	for i, date := range times {
		id, err := s.Reserve(ctx, cid, "private-workspace", 100, 0)
		if err != nil {
			t.Fatal(err)
		}
		if date == "" {
			continue
		}
		stamp, _ := time.Parse(time.RFC3339, date)
		cost := 0.0
		if i == 2 {
			cost = 0.4
		}
		if _, err = s.DB.Exec(`UPDATE research_runs SET model='test-model',started_at=?,status='failed',input_tokens=100,output_tokens=20,reasoning_tokens=5,cache_read_tokens=50,cache_write_tokens=2,cost_usd=? WHERE id=?`, stamp.Unix(), cost, id); err != nil {
			t.Fatal(err)
		}
	}
	buckets, err := s.AIUsage(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 3 {
		t.Fatalf("expected separate peak, off-peak, reported-cost buckets: %+v", buckets)
	}
	var runs, input, cache, output, reasoning int64
	var cost float64
	for _, b := range buckets {
		runs += b.Runs
		input += b.InputTokens
		cache += b.CacheReadTokens
		output += b.OutputTokens
		reasoning += b.ReasoningTokens
		cost += b.CostUSD
	}
	if runs != 5 || input != 500 || cache != 250 || output != 100 || reasoning != 25 || cost != 0.4 {
		t.Fatalf("incorrect totals: %+v", buckets)
	}
	since, _ := time.Parse(time.RFC3339, "2026-10-30T00:00:00Z")
	recent, err := s.AIUsage(ctx, since.Unix())
	if err != nil || len(recent) != 1 || recent[0].Runs != 2 {
		t.Fatal(recent, err)
	}
}
