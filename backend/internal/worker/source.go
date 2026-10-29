package worker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/safeurl"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/store"
)

func (w *Worker) loadSource(ctx context.Context, cid, raw string) (safeurl.Evidence, error) {
	if !w.Config.SourceBrowser {
		return safeurl.Fetch(ctx, raw)
	}
	capture := store.ID()
	dir := filepath.Join(w.Config.Workspaces, "sources", cid, capture)
	evidence, err := safeurl.FetchRendered(ctx, raw, dir, filepath.Join(w.Config.RepoPath, "scripts/source-browser.mjs"))
	if evidence.Rendered {
		evidence.CaptureID = capture
		_, dbErr := w.Store.DB.ExecContext(ctx, "INSERT INTO source_captures(id,case_id,url,artifact_dir,created_at) VALUES(?,?,?,?,?)", capture, cid, evidence.URL, evidence.ArtifactDir, time.Now().Unix())
		if dbErr != nil {
			return evidence, dbErr
		}
	}
	if errors.Is(err, safeurl.ErrBrowserUnavailable) {
		// Static pages can still be researched when the optional browser runtime is absent.
		plain, fetchErr := safeurl.Fetch(ctx, raw)
		plain.BrowserWarning = err.Error()
		_ = os.WriteFile(filepath.Join(dir, "capture-error.txt"), []byte(err.Error()), 0600)
		if fetchErr != nil {
			return plain, errors.New(err.Error() + "; HTTP fallback also failed: " + fetchErr.Error())
		}
		return plain, nil
	}
	return evidence, err
}
