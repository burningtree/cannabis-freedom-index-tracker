ALTER TABLE research_runs ADD COLUMN model TEXT NOT NULL DEFAULT '';
ALTER TABLE research_runs ADD COLUMN runner TEXT NOT NULL DEFAULT 'opencode';
ALTER TABLE research_runs ADD COLUMN session_id TEXT NOT NULL DEFAULT '';
ALTER TABLE research_runs ADD COLUMN status TEXT NOT NULL DEFAULT 'starting';
ALTER TABLE research_runs ADD COLUMN started_at INTEGER;
ALTER TABLE research_runs ADD COLUMN finished_at INTEGER;
ALTER TABLE research_runs ADD COLUMN input_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE research_runs ADD COLUMN output_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE research_runs ADD COLUMN cost_usd REAL NOT NULL DEFAULT 0;
CREATE INDEX research_runs_case ON research_runs(case_id,created_at,id);
CREATE TABLE ai_events (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 case_id TEXT NOT NULL REFERENCES cases(id),
 run_id TEXT NOT NULL REFERENCES research_runs(id),
 role TEXT NOT NULL CHECK(role IN ('user','assistant','tool','system')),
 kind TEXT NOT NULL, text TEXT NOT NULL, created_at INTEGER NOT NULL
);
CREATE INDEX ai_events_case ON ai_events(case_id,id);
CREATE TRIGGER submission_feed_ai_event AFTER INSERT ON ai_events BEGIN
 UPDATE submission_feed_version SET version=version+1 WHERE singleton=1;
END;
CREATE TRIGGER submission_feed_ai_run AFTER UPDATE OF status,model,session_id,input_tokens,output_tokens,cost_usd ON research_runs BEGIN
 UPDATE submission_feed_version SET version=version+1 WHERE singleton=1;
END;
