-- How the AI's answer was checked: which sources the service could open again, which quotations it
-- found, what was left out. Public, shown in the request page's outcome.
ALTER TABLE cases ADD COLUMN validation TEXT NOT NULL DEFAULT '';
