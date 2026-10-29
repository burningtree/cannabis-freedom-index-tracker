-- A durable version, not an event count. Triggers cover API, workers and operator writes.
CREATE TABLE submission_feed_version (singleton INTEGER PRIMARY KEY CHECK(singleton=1), version INTEGER NOT NULL);
INSERT INTO submission_feed_version VALUES(1,0);
CREATE TRIGGER submission_feed_insert AFTER INSERT ON submissions BEGIN
 UPDATE submission_feed_version SET version=version+1 WHERE singleton=1;
END;
CREATE TRIGGER submission_feed_links AFTER UPDATE OF gateway_event_id,reply_event_id ON submissions
WHEN OLD.gateway_event_id<>NEW.gateway_event_id OR OLD.reply_event_id<>NEW.reply_event_id BEGIN
 UPDATE submission_feed_version SET version=version+1 WHERE singleton=1;
END;
CREATE TRIGGER submission_feed_case AFTER UPDATE OF status,explanation,report_slug,pr_url ON cases
WHEN OLD.status<>NEW.status OR OLD.explanation<>NEW.explanation OR OLD.report_slug<>NEW.report_slug OR OLD.pr_url<>NEW.pr_url BEGIN
 UPDATE submission_feed_version SET version=version+1 WHERE singleton=1;
END;
CREATE TRIGGER submission_feed_history AFTER INSERT ON submission_history BEGIN
 UPDATE submission_feed_version SET version=version+1 WHERE singleton=1;
END;
