package worker

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/research"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/store"
)

// a job workspace is named after its case: <case id>-<random>
var workspaceName = regexp.MustCompile(`^([0-9a-f]{32})-[0-9]+$`)

// cleanWorkspaces keeps the job storage small. A workspace whose request has finished (committed,
// published, declined or failed) is emptied down to its logs and evidence; one whose request no
// longer exists is removed; one with a job running, or touched in the last ten minutes, is left
// alone, as are the source captures (sources/) the request pages show.
func (w *Worker) cleanWorkspaces(ctx context.Context) {
	root := w.Config.Workspaces
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	pruned, removed := 0, 0
	for _, e := range entries {
		m := workspaceName.FindStringSubmatch(e.Name())
		if !e.IsDir() || m == nil || ctx.Err() != nil {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if info, err := e.Info(); err != nil || time.Since(info.ModTime()) < 10*time.Minute {
			continue
		}
		var status string
		var running bool
		err := w.Store.DB.QueryRowContext(ctx, "SELECT status, EXISTS(SELECT 1 FROM jobs WHERE ref=cases.id AND status='running' AND lease_until>unixepoch()) FROM cases WHERE id=?", m[1]).Scan(&status, &running)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			if os.RemoveAll(dir) == nil {
				removed++
			}
		case err != nil || running:
			continue
		case store.Terminal(status):
			if pruneIfNeeded(dir) {
				pruned++
			}
		default:
			// still on its way (drafted, waiting to commit): only the build leftovers go
			research.PruneBuild(dir)
		}
	}
	if pruned+removed > 0 {
		slog.Info("job workspaces cleaned", "pruned", pruned, "removed", removed)
	}
}

// pruneIfNeeded empties a workspace unless it is already down to its logs; true when it did.
func pruneIfNeeded(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.Type().IsRegular() || !research.KeptFile(e.Name()) {
			return research.PruneWorkspace(dir) == nil
		}
	}
	return false
}

// janitor cleans the job storage at start and then every half hour.
func (w *Worker) janitor(ctx context.Context) {
	w.cleanWorkspaces(ctx)
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.cleanWorkspaces(ctx)
		}
	}
}
