package store

import "context"

// AIUsageBucket aggregates runs with the same pricing conditions. A representative
// start time preserves peak/off-peak estimation without exporting individual runs.
type AIUsageBucket struct {
	Model            string  `json:"model"`
	StartedAt        int64   `json:"started_at"`
	Runs             int64   `json:"runs"`
	Running          int64   `json:"running"`
	InputTokens      int64   `json:"input_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	ReasoningTokens  int64   `json:"reasoning_tokens"`
	CacheReadTokens  int64   `json:"cache_read_tokens"`
	CacheWriteTokens int64   `json:"cache_write_tokens"`
	CostUSD          float64 `json:"cost_usd"`
}

func (s *Store) AIUsage(ctx context.Context, since int64) ([]AIUsageBucket, error) {
	// Never join receipts: several submissions may share one research case/run.
	// Keep reported costs separate from token estimates, and split pricing tiers.
	rows, err := s.DB.QueryContext(ctx, `SELECT model,min(started_at),count(*),
 sum(CASE WHEN status='running' THEN 1 ELSE 0 END),
 sum(input_tokens),sum(output_tokens),sum(reasoning_tokens),
 sum(cache_read_tokens),sum(cache_write_tokens),sum(cost_usd)
 FROM research_runs WHERE started_at>0 AND started_at>=?
 GROUP BY model,cost_usd>0,
 (CAST(strftime('%w',started_at,'unixepoch') AS INTEGER) BETWEEN 1 AND 5
 AND (CAST(strftime('%H',started_at,'unixepoch') AS INTEGER) BETWEEN 1 AND 3
 OR CAST(strftime('%H',started_at,'unixepoch') AS INTEGER) BETWEEN 6 AND 9))
 ORDER BY model,min(started_at)`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AIUsageBucket{}
	for rows.Next() {
		var v AIUsageBucket
		if err := rows.Scan(&v.Model, &v.StartedAt, &v.Runs, &v.Running, &v.InputTokens, &v.OutputTokens, &v.ReasoningTokens, &v.CacheReadTokens, &v.CacheWriteTokens, &v.CostUSD); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
