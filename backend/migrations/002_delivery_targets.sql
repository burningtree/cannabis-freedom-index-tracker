CREATE TABLE IF NOT EXISTS nostr_delivery_targets (
 event_id TEXT NOT NULL REFERENCES nostr_events(id), relay TEXT NOT NULL,
 PRIMARY KEY(event_id,relay)
);
