-- What the service found on the submitted page before research, the AI's latest answer, and the
-- draft report built from it: all public, shown on the request page and its draft preview.
ALTER TABLE cases ADD COLUMN source_check TEXT NOT NULL DEFAULT '';
ALTER TABLE cases ADD COLUMN ai_result TEXT NOT NULL DEFAULT '';
ALTER TABLE cases ADD COLUMN draft_path TEXT NOT NULL DEFAULT '';
ALTER TABLE cases ADD COLUMN draft_markdown TEXT NOT NULL DEFAULT '';
