CREATE INDEX IF NOT EXISTS research_runs_case ON research_runs(case_id);
CREATE INDEX IF NOT EXISTS research_runs_created ON research_runs(created_at);
-- Repair receipts previously left at 'Source checked' while their job waited for budget.
UPDATE cases SET status='budget_wait',
 explanation='The daily AI research reservation limit has been reached. No new AI run has started; processing will resume after the budget reset.',
 updated_at=CAST(strftime('%s','now') AS INTEGER)
 WHERE status IN ('queued','classifying','fetching') AND id IN
 (SELECT ref FROM jobs WHERE kind='research' AND status='queued' AND error_code='daily_budget_wait');
INSERT INTO submission_history(submission_id,status,explanation,actor,created_at)
 SELECT s.id,c.status,c.explanation,'worker',c.updated_at FROM submissions s JOIN cases c ON c.id=s.case_id
 WHERE c.status='budget_wait';
