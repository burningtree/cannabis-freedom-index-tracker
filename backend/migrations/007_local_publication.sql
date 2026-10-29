CREATE TABLE local_publications (
 case_id TEXT PRIMARY KEY REFERENCES cases(id),
 commit_sha TEXT NOT NULL,
 report_path TEXT NOT NULL,
 created_at INTEGER NOT NULL
);
