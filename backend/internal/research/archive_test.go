package research

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/safeurl"
)

func TestIndependentlyOpenedArchiveIsSavedInReportSources(t *testing.T) {
	r := valid()
	archive := "https://web.archive.org/web/20200102030405id_/" + r.Sources[0].URL
	initial := safeurl.Evidence{URL: r.Sources[0].URL, FinalURL: archive, ArchiveURL: archive, Content: r.Facts[0].Quote}
	got, _, _, err := r.Confirm(context.Background(), initial, func(context.Context, string) (safeurl.Evidence, error) {
		t.Fatal("initial archive was unnecessarily reopened")
		return safeurl.Evidence{}, nil
	})
	if err != nil || got.Sources[0].Archived != archive || !strings.Contains(got.Markdown(), "archived: "+archive) {
		t.Fatal(got, err)
	}
	// Existing sources receive the verified archive link too.
	dir := t.TempDir()
	path := filepath.Join(dir, "reports/2026/01", r.Slug+".md")
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, []byte("---\ndate: 2026-01-01\ncountry: th\ntype: raid\nstatus: reported\nsources:\n  - title: News\n    url: "+r.Sources[0].URL+"\n    publisher: Publisher\n---\nOriginal report. [@1]\n"), 0644)
	got.Outcome = "existing_case"
	got.ExistingReport = got.Slug
	draft, err := PrepareDraft(dir, got)
	if err != nil || !strings.Contains(draft.Content, archive) {
		t.Fatal(draft, err)
	}
	if _, err := Strict([]byte(`{"sources":[{"url":"https://news.example/a","archived":"invented"}]}`)); err == nil {
		t.Fatal("model could supply an unverified archive link")
	}
}
