CREATE TABLE research_checkpoints (
 case_id TEXT PRIMARY KEY REFERENCES cases(id),
 payload TEXT NOT NULL,
 created_at INTEGER NOT NULL
);
CREATE TRIGGER submission_feed_job_insert AFTER INSERT ON jobs BEGIN
 UPDATE submission_feed_version SET version=version+1 WHERE singleton=1;
END;
CREATE TRIGGER submission_feed_job_status AFTER UPDATE OF status ON jobs BEGIN
 UPDATE submission_feed_version SET version=version+1 WHERE singleton=1;
END;
CREATE TRIGGER submission_feed_job_delete AFTER DELETE ON jobs BEGIN
 UPDATE submission_feed_version SET version=version+1 WHERE singleton=1;
END;
