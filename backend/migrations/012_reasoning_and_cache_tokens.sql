-- OpenCode reports reasoning and cached-input tokens apart from input and output. Reasoning is
-- billed as output and cached reads at their own (much lower) price, so a run's cost needs both.
-- tokens_backfilled marks runs whose counts were read again from their saved event log.
ALTER TABLE research_runs ADD COLUMN reasoning_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE research_runs ADD COLUMN cache_read_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE research_runs ADD COLUMN cache_write_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE research_runs ADD COLUMN tokens_backfilled INTEGER NOT NULL DEFAULT 0;
