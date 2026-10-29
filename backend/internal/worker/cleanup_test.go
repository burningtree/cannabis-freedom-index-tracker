package worker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/research"
)

// A finished request's workspace keeps its logs and evidence; the build leftovers and the
// repository copy go.
func TestPruneKeepsLogsOnly(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"research-events.jsonl", "research-stderr.log", "evidence-1-abc.json", "package.json", "node_modules/x/index.js", "dist/index.html", "reports/2026/10/a.md", ".research-input/source.txt"} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	research.PruneBuild(dir)
	for _, gone := range []string{"node_modules", "dist"} {
		if _, err := os.Stat(filepath.Join(dir, gone)); !os.IsNotExist(err) {
			t.Fatal(gone, "kept after PruneBuild")
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "reports/2026/10/a.md")); err != nil {
		t.Fatal("PruneBuild removed the draft")
	}
	if !pruneIfNeeded(dir) || pruneIfNeeded(dir) {
		t.Fatal("prune should run once")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 3 {
		t.Fatalf("kept %d entries", len(entries))
	}
}
