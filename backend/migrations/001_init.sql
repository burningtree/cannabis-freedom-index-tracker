CREATE TABLE IF NOT EXISTS cases (
 id TEXT PRIMARY KEY, canonical_url TEXT NOT NULL UNIQUE, status TEXT NOT NULL DEFAULT 'queued',
 explanation TEXT NOT NULL DEFAULT 'Waiting for research.', report_slug TEXT NOT NULL DEFAULT '', pr_url TEXT NOT NULL DEFAULT '',
 created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS submissions (
 id TEXT PRIMARY KEY, origin TEXT NOT NULL, original_url TEXT NOT NULL, canonical_url TEXT NOT NULL,
 note TEXT NOT NULL DEFAULT '', author TEXT NOT NULL DEFAULT '', origin_event_id TEXT UNIQUE,
 gateway_event_id TEXT NOT NULL DEFAULT '', reply_event_id TEXT NOT NULL DEFAULT '',
 case_id TEXT NOT NULL REFERENCES cases(id), created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS submissions_case ON submissions(case_id);
CREATE TABLE IF NOT EXISTS submission_history (
 id INTEGER PRIMARY KEY AUTOINCREMENT, submission_id TEXT NOT NULL REFERENCES submissions(id),
 status TEXT NOT NULL, explanation TEXT NOT NULL, actor TEXT NOT NULL, created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS history_submission ON submission_history(submission_id,id);
CREATE TABLE IF NOT EXISTS source_urls (url TEXT PRIMARY KEY, case_id TEXT NOT NULL REFERENCES cases(id), final_url TEXT NOT NULL DEFAULT '', digest TEXT NOT NULL DEFAULT '', fetched_at INTEGER);
CREATE TABLE IF NOT EXISTS jobs (
 id TEXT PRIMARY KEY, kind TEXT NOT NULL, ref TEXT NOT NULL, payload TEXT NOT NULL DEFAULT '{}',
 status TEXT NOT NULL DEFAULT 'queued', attempts INTEGER NOT NULL DEFAULT 0, available_at INTEGER NOT NULL,
 lease_until INTEGER NOT NULL DEFAULT 0, lease_token TEXT NOT NULL DEFAULT '', error_code TEXT NOT NULL DEFAULT '',
 UNIQUE(kind,ref)
);
CREATE INDEX IF NOT EXISTS jobs_ready ON jobs(status,available_at,lease_until);
CREATE TABLE IF NOT EXISTS nostr_events (id TEXT PRIMARY KEY, raw TEXT NOT NULL, direction TEXT NOT NULL, purpose TEXT NOT NULL, created_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS relay_receipts (event_id TEXT NOT NULL, relay TEXT NOT NULL, accepted INTEGER NOT NULL, attempts INTEGER NOT NULL DEFAULT 1, updated_at INTEGER NOT NULL, PRIMARY KEY(event_id,relay));
CREATE TABLE IF NOT EXISTS outbox (
 id TEXT PRIMARY KEY, kind TEXT NOT NULL, ref TEXT NOT NULL, payload TEXT NOT NULL,
 status TEXT NOT NULL DEFAULT 'queued', attempts INTEGER NOT NULL DEFAULT 0, available_at INTEGER NOT NULL,
 lease_until INTEGER NOT NULL DEFAULT 0, lease_token TEXT NOT NULL DEFAULT '', error_code TEXT NOT NULL DEFAULT '', UNIQUE(kind,ref)
);
CREATE INDEX IF NOT EXISTS outbox_ready ON outbox(status,available_at,lease_until);
CREATE TABLE IF NOT EXISTS harvest_state (provider TEXT PRIMARY KEY, cursor TEXT NOT NULL, updated_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS ingest_keys (key TEXT PRIMARY KEY, submission_id TEXT NOT NULL REFERENCES submissions(id));
CREATE TABLE IF NOT EXISTS research_runs (id TEXT PRIMARY KEY, case_id TEXT NOT NULL REFERENCES cases(id), workspace TEXT NOT NULL, outcome TEXT NOT NULL DEFAULT '', reserved_cents INTEGER NOT NULL, created_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS publication (case_id TEXT PRIMARY KEY REFERENCES cases(id), branch TEXT NOT NULL, slug TEXT NOT NULL, pr_number INTEGER, head_sha TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL);
