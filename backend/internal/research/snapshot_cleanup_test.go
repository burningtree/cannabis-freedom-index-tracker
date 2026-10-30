package research

import (
	"context"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/config"
	"os"
	"path/filepath"
	"testing"
)

func TestFailedSnapshotRemovesPartialCheckout(t *testing.T) {
	root := t.TempDir()
	runner := Runner{Config: config.Config{Workspaces: root, RepoPath: filepath.Join(root, "missing-repo")}}
	if _, err := runner.Snapshot(context.Background(), "0123456789abcdef0123456789abcdef"); err == nil {
		t.Fatal("expected clone error")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("partial snapshot leaked", entries, err)
	}
}
