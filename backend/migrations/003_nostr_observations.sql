CREATE TABLE IF NOT EXISTS nostr_observations (
 event_id TEXT NOT NULL REFERENCES nostr_events(id), relay TEXT NOT NULL,
 first_seen INTEGER NOT NULL, last_seen INTEGER NOT NULL,
 PRIMARY KEY(event_id,relay)
);
