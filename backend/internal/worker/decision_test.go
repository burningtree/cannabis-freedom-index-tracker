package worker

import (
	"context"
	"encoding/json"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/research"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/store"
	"path/filepath"
	"strings"
	"testing"
)

func TestOverlapReasonKeepsModelAssessmentAndPublicLinks(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "decisions.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	ctx := context.Background()
	old, err := s.Accept(ctx, store.Intake{Origin: "web", URL: "https://news.example/old", CanonicalURL: "https://news.example/old"})
	if err != nil {
		t.Fatal(err)
	}
	current, err := s.Accept(ctx, store.Intake{Origin: "web", URL: "https://news.example/new", CanonicalURL: "https://news.example/new"})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := s.Get(ctx, old)
	b, _ := s.Get(ctx, current)
	if err = s.Transition(ctx, a.CaseID, "duplicate", "Already covered by an existing report.", "existing-report", ""); err != nil {
		t.Fatal(err)
	}
	assessment := `{"explanation":"The article supports a new report.","decision_title":"A supported sentencing event"}`
	s.SetCaseOutput(ctx, b.CaseID, map[string]string{"ai_result": assessment})
	w := Worker{Store: s}
	if err = w.rejectSourceOverlap(ctx, b.CaseID, a.CaseID, "https://news.example/shared", research.Validation{Passed: true}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, current)
	if err != nil {
		t.Fatal(err)
	}
	var validation struct {
		SourceOverlap struct {
			URL          string `json:"url"`
			SubmissionID string `json:"submission_id"`
			ReportSlug   string `json:"report_slug"`
		} `json:"source_overlap"`
	}
	if err = json.Unmarshal(got.Validation, &validation); err != nil {
		t.Fatal(err)
	}
	if got.Status != "insufficient_evidence" || len(got.Explanation) < 200 || !strings.Contains(got.Explanation, "service") || validation.SourceOverlap.SubmissionID != old || validation.SourceOverlap.ReportSlug != "existing-report" || string(got.Result) != assessment {
		t.Fatal(got, validation)
	}
}
