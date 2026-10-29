package store

import (
	"context"
	"time"
)

// ResumeBudgetWait is called under the worker lock on startup when the daily cap
// is disabled. Change only budget waits, keeping evidence, usage and other delays.
func (s *Store) ResumeBudgetWait(ctx context.Context) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().Unix()
	const explanation = "The daily research limit is now unlimited. Queued to resume automatically."
	const eligible = `SELECT c.id FROM cases c JOIN jobs j ON j.ref=c.id WHERE c.status='budget_wait' AND j.kind='research' AND j.status='queued' AND j.error_code='daily_budget_wait'`
	if _, err = tx.ExecContext(ctx, `INSERT INTO submission_history(submission_id,status,explanation,actor,created_at) SELECT id,'queued',?,'worker',? FROM submissions WHERE case_id IN (`+eligible+`)`, explanation, now); err != nil {
		return err
	}
	// Updating cases first preserves the eligible job set for the following update.
	if _, err = tx.ExecContext(ctx, `UPDATE cases SET status='queued',explanation=?,updated_at=? WHERE id IN (`+eligible+`)`, explanation, now); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE jobs SET available_at=?,error_code='' WHERE kind='research' AND status='queued' AND error_code='daily_budget_wait' AND ref IN (SELECT id FROM cases WHERE status='queued')`, now); err != nil {
		return err
	}
	return tx.Commit()
}
