-- Keep old receipt/history links intact while making URL reuse efficient.
CREATE INDEX IF NOT EXISTS submissions_canonical_lookup
ON submissions(canonical_url, created_at, id);
