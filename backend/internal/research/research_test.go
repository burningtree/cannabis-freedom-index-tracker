package research

import (
	"context"
	"encoding/json"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/safeurl"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func valid() Result {
	return Result{Outcome: "new_event", Country: "th", Date: "2026-01-02", Type: "raid", Slug: "example-raid", Title: "Police raid a cannabis farm", Summary: "Police raided a farm.", Explanation: "A sourced enforcement incident.", Sources: []Source{{"https://news.example/a", "News story", "Publisher"}}, Facts: []Fact{{"Police raided the farm.", 1, "Police raided the farm in the province."}}}
}
func TestStrictAndValidation(t *testing.T) {
	r := valid()
	b, _ := json.Marshal(r)
	parsed, err := Strict(b)
	if err != nil || parsed.Validate(map[string]bool{"th": true}) != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Result){func(r *Result) { r.Country = "invented" }, func(r *Result) { r.Date = "1999-01-01" }, func(r *Result) { r.Slug = "../../secret" }, func(r *Result) { r.Facts[0].Source = 2 }, func(r *Result) { r.Type = "execution" }, func(r *Result) { r.Facts[0].Text = "<script>steal()</script>" }} {
		v := valid()
		mutate(&v)
		if v.Validate(map[string]bool{"th": true}) == nil {
			t.Fatal(v)
		}
	}
	if _, err = Strict([]byte(`{"outcome":"needs_review","command":"push main"}`)); err == nil {
		t.Fatal("unknown fields accepted")
	}
}
func TestQuoteMustBeInFetchedEvidence(t *testing.T) {
	r := valid()
	initial := safeurl.Evidence{URL: r.Sources[0].URL, Content: "Unrelated article"}
	if _, err := r.Verify(context.Background(), initial); err == nil {
		t.Fatal("hallucinated quote accepted")
	}
	initial.Content = r.Facts[0].Quote
	if _, err := r.Verify(context.Background(), initial); err != nil {
		t.Fatal(err)
	}
}
func TestExecutionStreamIsNotResult(t *testing.T) {
	r := valid()
	b, _ := json.Marshal(r)
	event, _ := json.Marshal(map[string]any{"type": "text", "part": map[string]string{"text": string(b)}})
	final, err := FinalText(append(event, '\n'))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Strict(final); err != nil {
		t.Fatal(err)
	}
	if _, err = FinalText([]byte(`{"type":"step_finish"}`)); err == nil {
		t.Fatal("partial output accepted")
	}
	if _, err = FinalText([]byte(`{"type":"error"}`)); err == nil {
		t.Fatal("error ignored")
	}
}
func TestExistingCaseDraftPreservesCitationsAndSlug(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "reports/2026/01/example-raid.md")
	if err := os.MkdirAll(filepath.Dir(oldPath), 0755); err != nil {
		t.Fatal(err)
	}
	original := `---
date: 2026-01-02
country: th
type: arrest
title: Earlier arrest
summary: Earlier reporting.
status: verified
sources:
  - url: https://news.example/earlier
    title: Earlier story
    publisher: Publisher
---

Police arrested a person. [@1]
`
	if err := os.WriteFile(oldPath, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	r := valid()
	r.Outcome = "existing_case"
	r.ExistingReport = r.Slug
	r.Date = "2026-02-03"
	r.Type = "sentence"
	if err := r.Validate(map[string]bool{"th": true}); err != nil {
		t.Fatal(err)
	}
	d, err := PrepareDraft(dir, r)
	if err != nil {
		t.Fatal(err)
	}
	if d.Path != "reports/2026/02/example-raid.md" || d.PreviousPath != "reports/2026/01/example-raid.md" || !strings.Contains(d.Content, "Police arrested a person. [@1]") || !strings.Contains(d.Content, r.Facts[0].Text+" [@2]") {
		t.Fatal(d)
	}
	parts := strings.SplitN(d.Content, "---", 3)
	var header map[string]any
	if err = yaml.Unmarshal([]byte(parts[1]), &header); err != nil {
		t.Fatal(err)
	}
	if header["status"] != "reported" || header["date"] != "2026-02-03" || header["type"] != "sentence" {
		t.Fatal(header)
	}
	r.Country = "us"
	if _, err = PrepareDraft(dir, r); err == nil {
		t.Fatal("cross-country match accepted")
	}
	r.Country = "th"
	r.Date = "2025-12-01"
	if _, err = PrepareDraft(dir, r); err == nil {
		t.Fatal("earlier step replaced latest date")
	}
}
