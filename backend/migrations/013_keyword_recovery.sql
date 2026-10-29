-- Only old automatic keyword rejections with no AI research are retried.
-- Deliberate AI outside-scope outcomes remain untouched.
CREATE TEMP TABLE keyword_recovery_cases AS
 SELECT id FROM cases WHERE status='not_relevant'
 AND explanation LIKE 'The submitted page (%does not mention cannabis.'
 AND NOT EXISTS (SELECT 1 FROM research_runs r WHERE r.case_id=cases.id)
 AND NOT EXISTS (SELECT 1 FROM jobs j WHERE j.ref=cases.id AND j.status='running');
INSERT INTO jobs(id,kind,ref,payload,status,available_at)
 SELECT lower(hex(randomblob(16))),'research',id,'{}','queued',unixepoch() FROM keyword_recovery_cases WHERE 1
 ON CONFLICT(kind,ref) DO UPDATE SET status='queued',attempts=0,available_at=excluded.available_at,lease_until=0,lease_token='',error_code='';
INSERT INTO submission_history(submission_id,status,explanation,actor,created_at)
 SELECT id,'queued','The multilingual source vocabulary was updated. Queued for AI relevance checking.','recovery',unixepoch()
 FROM submissions WHERE case_id IN (SELECT id FROM keyword_recovery_cases);
UPDATE cases SET status='queued',explanation='The multilingual source vocabulary was updated. Queued for AI relevance checking.',updated_at=unixepoch()
 WHERE id IN (SELECT id FROM keyword_recovery_cases);
DELETE FROM outbox WHERE kind='resolution' AND status<>'done'
 AND ref IN (SELECT id||':not_relevant' FROM submissions WHERE case_id IN (SELECT id FROM keyword_recovery_cases));
DROP TABLE keyword_recovery_cases;
