-- The submissions register pages by the latest case update.
CREATE INDEX IF NOT EXISTS cases_updated_at ON cases(updated_at);
