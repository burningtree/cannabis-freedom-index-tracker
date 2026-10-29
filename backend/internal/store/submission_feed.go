package store

import (
	"context"
	"errors"
	"strconv"
)

const SubmissionPageSize = 50

type SubmissionEntry struct {
	LastStage    string `json:"last_stage,omitempty"`
	ResumeAt     int64  `json:"resume_at,omitempty"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	RunCount     int64  `json:"run_count"`
	ID           string `json:"id"`
	Origin       string `json:"origin"`
	URL          string `json:"url"`
	Note         string `json:"note"`
	CaseID       string `json:"case_id"`
	Status       string `json:"status"`
	Explanation  string `json:"explanation"`
	ReportSlug   string `json:"report_slug,omitempty"`
	PRURL        string `json:"pr_url,omitempty"`
	NostrEventID string `json:"nostr_event_id,omitempty"`
	ReplyEventID string `json:"reply_event_id,omitempty"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
	sequence     int64
}
type SubmissionSummary struct {
	Total   int64 `json:"total"`
	Running int64 `json:"running"`
	Queued  int64 `json:"queued"`
	Paused  int64 `json:"paused"`
}

type SubmissionPage struct {
	Working     []SubmissionEntry `json:"working"`
	Summary     SubmissionSummary `json:"summary"`
	Submissions []SubmissionEntry `json:"submissions"`
	Total       int64             `json:"total"`
	NextCursor  string            `json:"next_cursor"`
	Revision    string            `json:"revision"`
}

func ValidSubmissionFilter(status, origin string) bool {
	return (status == "" || states[status]) && (origin == "" || origin == "web" || origin == "nostr" || origin == "rss" || origin == "gdelt" || origin == "api")
}
func (s *Store) FeedVersion(ctx context.Context) (string, error) {
	var v int64
	err := s.DB.QueryRowContext(ctx, "SELECT version FROM submission_feed_version WHERE singleton=1").Scan(&v)
	return strconv.FormatInt(v, 10), err
}

// ListSubmissions returns a bounded public projection of every intake origin. Cursor order
// uses insertion sequence so same-second receipts and later status changes cannot skip rows.
func (s *Store) ListSubmissions(ctx context.Context, cursor int64, status, origin string) (SubmissionPage, error) {
	out := SubmissionPage{Submissions: []SubmissionEntry{}, Working: []SubmissionEntry{}}
	if cursor < 1 || !ValidSubmissionFilter(status, origin) {
		return out, errors.New("invalid submission filters")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if err = tx.QueryRowContext(ctx, `SELECT
      (SELECT count(*) FROM submissions),
      count(DISTINCT CASE WHEN j.status='running' AND j.lease_until>unixepoch() THEN c.id END),
      count(DISTINCT CASE WHEN j.status='queued' AND c.status<>'budget_wait' AND NOT EXISTS(SELECT 1 FROM jobs active WHERE active.ref=c.id AND active.status='running' AND active.lease_until>unixepoch()) THEN c.id END),
      count(DISTINCT CASE WHEN c.status='budget_wait' THEN c.id END)
      FROM cases c LEFT JOIN jobs j ON j.ref=c.id
      WHERE c.status IN ('queued','budget_wait','fetching','classifying','researching','verifying','drafted','awaiting_review','publishing','merged','deploying')`).Scan(&out.Summary.Total, &out.Summary.Running, &out.Summary.Queued, &out.Summary.Paused); err != nil {
		return out, err
	}
	var revision int64
	if err = tx.QueryRowContext(ctx, "SELECT version FROM submission_feed_version WHERE singleton=1").Scan(&revision); err != nil {
		return out, err
	}
	out.Revision = strconv.FormatInt(revision, 10)
	where := " WHERE 1=1"
	args := []any{}
	if status != "" {
		where += " AND c.status=?"
		args = append(args, status)
	}
	if origin != "" {
		where += " AND s.origin=?"
		args = append(args, origin)
	}
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM submissions s JOIN cases c ON c.id=s.case_id"+where, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	args = append(args, cursor)
	projection := `SELECT s.rowid,s.id,s.origin,s.original_url,s.note,s.case_id,c.status,c.explanation,c.report_slug,c.pr_url,coalesce(s.origin_event_id,nullif(s.gateway_event_id,''),''),s.reply_event_id,s.created_at,c.updated_at,
 coalesce((SELECT status FROM submission_history WHERE submission_id=s.id AND status IN ('queued','fetching','classifying','researching','verifying','drafted','publishing','committed','published') ORDER BY id DESC LIMIT 1),''),
 coalesce((SELECT sum(input_tokens) FROM research_runs WHERE case_id=c.id),0),
 coalesce((SELECT sum(output_tokens) FROM research_runs WHERE case_id=c.id),0),
 (SELECT count(*) FROM research_runs WHERE case_id=c.id),
 coalesce((SELECT available_at FROM jobs WHERE ref=c.id AND kind='research' AND error_code='daily_budget_wait' AND status='queued'),0)
 FROM submissions s JOIN cases c ON c.id=s.case_id`
	rows, err := tx.QueryContext(ctx, projection+where+` AND s.rowid<? ORDER BY s.rowid DESC LIMIT 51`, args...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var v SubmissionEntry
		if err = rows.Scan(&v.sequence, &v.ID, &v.Origin, &v.URL, &v.Note, &v.CaseID, &v.Status, &v.Explanation, &v.ReportSlug, &v.PRURL, &v.NostrEventID, &v.ReplyEventID, &v.CreatedAt, &v.UpdatedAt, &v.LastStage, &v.InputTokens, &v.OutputTokens, &v.RunCount, &v.ResumeAt); err != nil {
			rows.Close()
			return out, err
		}
		out.Submissions = append(out.Submissions, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if len(out.Submissions) > SubmissionPageSize {
		out.Submissions = out.Submissions[:SubmissionPageSize]
		out.NextCursor = strconv.FormatInt(out.Submissions[SubmissionPageSize-1].sequence, 10)
	}
	active, err := tx.QueryContext(ctx, projection+` WHERE
      c.status IN ('queued','fetching','classifying','researching','verifying','drafted','awaiting_review','publishing','merged','deploying')
      AND s.rowid=(SELECT max(latest.rowid) FROM submissions latest WHERE latest.case_id=c.id)
      AND EXISTS(SELECT 1 FROM jobs j WHERE j.ref=c.id AND j.status='running' AND j.lease_until>unixepoch())
      ORDER BY c.updated_at DESC,s.rowid DESC LIMIT 50`)
	if err != nil {
		return out, err
	}
	for active.Next() {
		var v SubmissionEntry
		if err = active.Scan(&v.sequence, &v.ID, &v.Origin, &v.URL, &v.Note, &v.CaseID, &v.Status, &v.Explanation, &v.ReportSlug, &v.PRURL, &v.NostrEventID, &v.ReplyEventID, &v.CreatedAt, &v.UpdatedAt, &v.LastStage, &v.InputTokens, &v.OutputTokens, &v.RunCount, &v.ResumeAt); err != nil {
			active.Close()
			return out, err
		}
		out.Working = append(out.Working, v)
	}
	err = active.Err()
	active.Close()
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
