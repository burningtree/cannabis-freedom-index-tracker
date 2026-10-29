package research

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/safeurl"
	"gopkg.in/yaml.v3"
)

func TestOpenQuestionsDoNotInvalidateSourcedReportAndSurviveMarkdown(t *testing.T) {
	r := valid()
	r.Uncertainties = []string{"Outlets disagree on whether the truck carried cabbage or cauliflower.", "The court outcome is not yet known."}
	if err := r.Validate(map[string]bool{"th": true}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Verify(context.Background(), safeurl.Evidence{URL: r.Sources[0].URL, Content: r.Facts[0].Quote}); err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(r.Markdown(), "---", 3)
	var header struct {
		Questions []string `yaml:"open_questions"`
		Status    string   `yaml:"status"`
	}
	if err := yaml.Unmarshal([]byte(parts[1]), &header); err != nil {
		t.Fatal(err)
	}
	if len(header.Questions) != 2 || header.Questions[0] != r.Uncertainties[0] || header.Status != "reported" {
		t.Fatal(header)
	}
	r.Facts[0].Quote = "Unsupported central factual claim."
	if _, err := r.Verify(context.Background(), safeurl.Evidence{URL: r.Sources[0].URL, Content: "Nothing supporting that claim."}); err == nil {
		t.Fatal("open questions bypassed source verification")
	}
}
func TestExistingCaseRetainsAndAddsOpenQuestions(t *testing.T) {
	dir := t.TempDir()
	r := valid()
	r.Outcome = "existing_case"
	r.ExistingReport = r.Slug
	path := filepath.Join(dir, "reports/2026/01", r.Slug+".md")
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, []byte("---\ndate: 2026-01-01\ncountry: th\ntype: raid\nstatus: reported\nsources:\n  - title: News\n    url: https://news.example/a\n    publisher: Publisher\nopen_questions:\n  - Previous question.\n---\nOriginal prose. [@1]\n"), 0644)
	r.Uncertainties = []string{"Previous question.", "New question."}
	d, err := PrepareDraft(dir, r)
	if err != nil {
		t.Fatal(err)
	}
	var header struct {
		Questions []string `yaml:"open_questions"`
	}
	if err = yaml.Unmarshal([]byte(strings.SplitN(d.Content, "---", 3)[1]), &header); err != nil {
		t.Fatal(err)
	}
	if len(header.Questions) != 2 || !strings.Contains(d.Content, "Original prose.") {
		t.Fatal(d.Content)
	}
}
func TestSnapshotUsesCurrentSiteInputsWithoutCopyingCredentials(t *testing.T) {
	repo := t.TempDir()
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatal(err, string(out))
		}
	}
	git("init")
	git("config", "user.name", "Fixture")
	git("config", "user.email", "fixture@localhost")
	os.MkdirAll(filepath.Join(repo, "src"), 0755)
	os.WriteFile(filepath.Join(repo, "src/schema.txt"), []byte("old schema"), 0644)
	git("add", ".")
	git("commit", "-m", "Initial")
	os.WriteFile(filepath.Join(repo, "src/schema.txt"), []byte("new schema with questions"), 0644)
	os.WriteFile(filepath.Join(repo, "src/new-renderer.txt"), []byte("new component"), 0644)
	os.WriteFile(filepath.Join(repo, ".env"), []byte("secret"), 0600)
	if err := snapshotSite(context.Background(), repo, dir); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "src/schema.txt"))
	if string(data) != "new schema with questions" {
		t.Fatal("snapshot used stale committed code")
	}
	if _, err := os.Stat(filepath.Join(dir, "src/new-renderer.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".env")); !os.IsNotExist(err) {
		t.Fatal("copied host credentials")
	}
}
