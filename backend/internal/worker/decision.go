package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/research"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/store"
)

// This is a service validation stop, not a model rejection. Keep its context
// separate from the saved model assessment, and link only public source/receipt data.
func (w *Worker) rejectSourceOverlap(ctx context.Context, cid, other, source string, check research.Validation) error {
	var receipt, report, status string
	if err := w.Store.DB.QueryRowContext(ctx, `SELECT s.id,c.report_slug,c.status FROM submissions s JOIN cases c ON c.id=s.case_id WHERE c.id=? ORDER BY s.created_at,s.id LIMIT 1`, other).Scan(&receipt, &report, &status); err != nil {
		return err
	}
	var validation map[string]any
	if err := json.Unmarshal([]byte(store.JSON(check)), &validation); err != nil {
		return err
	}
	validation["source_overlap"] = map[string]string{"url": source, "submission_id": receipt, "report_slug": report, "status": status}
	if err := w.Store.SetCaseOutput(ctx, cid, map[string]string{"validation": store.JSON(validation)}); err != nil {
		return err
	}
	reason := fmt.Sprintf("Publication stopped because an opened source is already attached to another submission, whose current status is %s. The service could not establish whether this is the same event, a later development, or separate events covered by the same article. It left the existing case intact and did not create or update a report. Compare the shared source and related submission below; a source that clearly identifies the event or its relationship to the existing case is needed to resolve the overlap.", status)
	return w.Store.Transition(ctx, cid, w.uncertainOutcome(), reason, "", "")
}
