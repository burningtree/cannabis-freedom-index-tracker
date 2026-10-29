package store

import (
	"context"
	"errors"
	"time"
)

var ErrRestartConflict = errors.New("submission cannot be restarted in its current state")

func Restartable(status string) bool {
	return status == "failed" || status == "insufficient_evidence" || status == "source_unavailable" || status == "needs_review"
}

// Restart a shared case atomically, fencing old leases and retaining all previous
// history and research runs. Double clicks cannot queue duplicate attempts.
func (s *Store) Restart(ctx context.Context, id string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var cid, status string
	if err = tx.QueryRowContext(ctx, "SELECT c.id,c.status FROM cases c JOIN submissions s ON s.case_id=c.id WHERE s.id=?", id).Scan(&cid, &status); err != nil {
		return err
	}
	if !Restartable(status) {
		return ErrRestartConflict
	}
	now := time.Now().Unix()
	var active int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM jobs WHERE ref=? AND status='running' AND lease_until>?", cid, now).Scan(&active); err != nil {
		return err
	}
	if active > 0 {
		return ErrRestartConflict
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM research_checkpoints WHERE case_id=?", cid); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM jobs WHERE ref=? AND kind='publish'", cid); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO jobs(id,kind,ref,payload,available_at) VALUES(?,'research',?,'{}',?)
 ON CONFLICT(kind,ref) DO UPDATE SET status='queued',attempts=0,available_at=excluded.available_at,lease_until=0,lease_token='',error_code=''`, ID(), cid, now); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE cases SET status='queued',explanation='Restart requested locally. Waiting for research.',report_slug='',pr_url='',source_check='',ai_result='',draft_path='',draft_markdown='',updated_at=? WHERE id=?", now, cid); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO submission_history(submission_id,status,explanation,actor,created_at) SELECT id,'queued','Restart requested locally. Waiting for research.','local',? FROM submissions WHERE case_id=?", now, cid); err != nil {
		return err
	}
	return tx.Commit()
}
