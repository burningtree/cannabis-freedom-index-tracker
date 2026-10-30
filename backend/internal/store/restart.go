package store

import (
	"context"
	"errors"
	"time"
)

var ErrRestartConflict = errors.New("submission cannot be restarted in its current state")

// ErrNothingToResume is a retry of a submission whose research did not finish: restart it instead.
var ErrNothingToResume = errors.New("submission has no finished research to resume from")

func Restartable(status string) bool {
	return status == "budget_wait" || status == "failed" || status == "insufficient_evidence" || status == "source_unavailable" || status == "needs_review"
}

// Restart a shared case atomically, fencing old leases and retaining all previous
// history and research runs. Double clicks cannot queue duplicate attempts.
func (s *Store) Restart(ctx context.Context, id string) error { return s.requeue(ctx, id, false) }

// Retry resumes a case after its finished AI research: the saved answer is checked against its
// sources, built and committed again, with no new AI run. Only a case with saved research can.
func (s *Store) Retry(ctx context.Context, id string) error { return s.requeue(ctx, id, true) }

func (s *Store) requeue(ctx context.Context, id string, resume bool) error {
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
	explanation, outputs := "Restart requested locally. Waiting for research.", ",source_check='',ai_result='',validation='',draft_path='',draft_markdown=''"
	if resume {
		var saved int
		if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM research_checkpoints WHERE case_id=?", cid).Scan(&saved); err != nil {
			return err
		}
		if saved == 0 {
			return ErrNothingToResume
		}
		// the research and what it found stay; the checks and the draft are redone
		explanation, outputs = "Retry requested locally. Resuming after the AI research.", ",validation='',draft_path='',draft_markdown=''"
	} else if _, err = tx.ExecContext(ctx, "DELETE FROM research_checkpoints WHERE case_id=?", cid); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM jobs WHERE ref=? AND kind='publish'", cid); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO jobs(id,kind,ref,payload,available_at) VALUES(?,'research',?,'{}',?)
 ON CONFLICT(kind,ref) DO UPDATE SET status='queued',attempts=0,available_at=excluded.available_at,lease_until=0,lease_token='',error_code=''`, ID(), cid, now); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE cases SET status='queued',explanation=?,report_slug='',pr_url=''"+outputs+",updated_at=? WHERE id=?", explanation, now, cid); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO submission_history(submission_id,status,explanation,actor,created_at) SELECT id,'queued',?,'local',? FROM submissions WHERE case_id=?", explanation, now, cid); err != nil {
		return err
	}
	return tx.Commit()
}
