package worker

import (
	"context"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/config"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/store"
	"path/filepath"
	"testing"
	"time"
)

func TestDemoReceiptProgressPersistsWithoutResearch(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "demo.sqlite")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.Accept(ctx, store.Intake{Origin: "web", URL: "https://news.example/article", CanonicalURL: "https://news.example/article"})
	if err != nil {
		t.Fatal(err)
	}
	j, err := s.Claim(ctx, "jobs", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	w := Worker{Store: s, Config: config.Config{Demo: true}}
	if err = w.process(ctx, j); err != nil {
		t.Fatal(err)
	}
	if err = s.Finish(ctx, "jobs", j, nil); err != nil {
		t.Fatal(err)
	}
	s.DB.Close()
	s, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	v, err := s.Get(ctx, id)
	if err != nil || v.Status != "insufficient_evidence" || len(v.History) != 5 {
		t.Fatal(v, err)
	}
	if v.PRURL != "" || v.ReportSlug != "" {
		t.Fatal("demo fabricated a publication")
	}
}
