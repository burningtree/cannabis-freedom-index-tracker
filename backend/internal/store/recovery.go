package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

var ErrInterrupted = errors.New("server interrupted processing")

// LockWorker must be held before Recover and until all workers have stopped.
// A process crash releases flock automatically, including after SIGKILL.
func LockWorker(database string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(database), 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(database+".worker.lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("another backend owns this database: %w", err)
	}
	return f, nil
}

// Recover is called only under the exclusive process lock, before workers start.
// Preserve checkpoints, publication intents, reservations and outbox payloads.
func (s *Store) Recover(ctx context.Context) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().Unix()
	for _, table := range []string{"jobs", "outbox"} {
		if _, err = tx.ExecContext(ctx, "UPDATE "+table+" SET status='queued',available_at=?,lease_until=0,lease_token='',attempts=max(0,attempts-1),error_code='server_interrupted' WHERE status='running'", now); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO ai_events(case_id,run_id,role,kind,text,created_at)
 SELECT case_id,id,'system','message','The server stopped during this AI run. Processing will recover automatically; earlier activity and usage are retained.',? FROM research_runs WHERE status IN ('starting','preparing','running')`, now); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE research_runs SET status='interrupted',finished_at=? WHERE status IN ('starting','preparing','running')", now); err != nil {
		return err
	}
	// Repair orphaned intermediate cases as well as leases. A durable publish intent wins
	// over research; never repeat the model or lose a commit that already happened.
	if _, err = tx.ExecContext(ctx, `INSERT INTO jobs(id,kind,ref,payload,status,available_at)
 SELECT lower(hex(randomblob(16))),'research',c.id,'{}','queued',? FROM cases c
 WHERE c.status IN ('queued','fetching','classifying','researching','drafted','awaiting_review','publishing','merged','deploying')
 AND NOT EXISTS(SELECT 1 FROM jobs p WHERE p.ref=c.id AND p.kind='publish')
 ON CONFLICT(kind,ref) DO UPDATE SET status='queued',available_at=excluded.available_at,lease_until=0,lease_token='',error_code='server_recovered'
 WHERE jobs.status='done'`, now); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO submission_history(submission_id,status,explanation,actor,created_at)
 SELECT s.id,'queued','Recovering automatically after the server stopped. Saved research and publication work will be resumed.','recovery',?
 FROM submissions s JOIN cases c ON c.id=s.case_id WHERE c.status IN ('fetching','classifying','researching','drafted','awaiting_review','publishing','merged','deploying')
 AND EXISTS(SELECT 1 FROM jobs j WHERE j.ref=c.id AND j.status='queued')`, now); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE cases SET status='queued',explanation='Recovering automatically after the server stopped. Saved research and publication work will be resumed.',updated_at=?
 WHERE status IN ('fetching','classifying','researching','drafted','awaiting_review','publishing','merged','deploying')
 AND EXISTS(SELECT 1 FROM jobs j WHERE j.ref=cases.id AND j.status='queued')`, now); err != nil {
		return err
	}
	return tx.Commit()
}
