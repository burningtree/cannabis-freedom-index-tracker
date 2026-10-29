CREATE TABLE source_captures (
 id TEXT PRIMARY KEY,
 case_id TEXT NOT NULL REFERENCES cases(id),
 url TEXT NOT NULL,
 artifact_dir TEXT NOT NULL,
 created_at INTEGER NOT NULL
);
CREATE INDEX source_captures_case ON source_captures(case_id,url,created_at);
