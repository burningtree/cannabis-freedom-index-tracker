package publisher

import (
	"context"
	"errors"
	"fmt"
)

const ValidationCheck = "Validate report"

// Automatic publication uses GitHub's ordinary merge endpoint with the exact
// validated head SHA. It never bypasses branch protection or failed CI checks.
func (c *Client) automaticMerge(ctx context.Context, cid string, p pull) (bool, error) {
	if p.Merged {
		return true, nil
	}
	var expected string
	if err := c.Store.DB.QueryRowContext(ctx, "SELECT head_sha FROM publication WHERE case_id=?", cid).Scan(&expected); err != nil {
		return false, err
	}
	if expected == "" || p.Head.SHA != expected || p.Base.Ref != "main" {
		return false, c.Store.Transition(ctx, cid, "failed", "Automatic publication stopped because the proposal differs from the validated commit.", "", p.URL)
	}
	if p.Draft {
		return false, c.Store.Transition(ctx, cid, "failed", "This older proposal is a draft. Automatic publication requires a normal proposal with passing checks.", "", p.URL)
	}
	var checks struct {
		Total int `json:"total_count"`
		Runs  []struct {
			Name       string `json:"name"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
		} `json:"check_runs"`
	}
	if err := c.api(ctx, "GET", fmt.Sprintf("/commits/%s/check-runs?per_page=100", expected), nil, &checks); err != nil {
		return false, err
	}
	if checks.Total > 100 {
		return false, errors.New("too many commit checks to validate automatically")
	}
	validated := false
	for _, check := range checks.Runs {
		if check.Status != "completed" {
			return false, nil
		}
		if check.Conclusion != "success" && check.Conclusion != "neutral" && check.Conclusion != "skipped" {
			return false, c.Store.Transition(ctx, cid, "failed", "Automatic publication stopped because a commit check failed.", "", p.URL)
		}
		if check.Name == ValidationCheck && check.Conclusion == "success" {
			validated = true
		}
	}
	if !validated {
		return false, nil
	}
	var statuses struct {
		State string `json:"state"`
		Total int    `json:"total_count"`
	}
	if err := c.api(ctx, "GET", fmt.Sprintf("/commits/%s/status", expected), nil, &statuses); err != nil {
		return false, err
	}
	if statuses.Total > 0 && statuses.State != "success" {
		if statuses.State == "failure" || statuses.State == "error" {
			return false, c.Store.Transition(ctx, cid, "failed", "Automatic publication stopped because a commit status failed.", "", p.URL)
		}
		return false, nil
	}
	var merged struct {
		Merged bool `json:"merged"`
	}
	err := c.api(ctx, "PUT", fmt.Sprintf("/pulls/%d/merge", p.Number), map[string]string{"sha": expected, "merge_method": "squash"}, &merged)
	if err != nil {
		var failure APIError
		if errors.As(err, &failure) && (failure.Status == 403 || failure.Status == 405 || failure.Status == 409 || failure.Status == 422) {
			return false, c.Store.Transition(ctx, cid, "failed", "GitHub could not merge the validated report automatically. Check repository rules, merge conflicts and App permissions.", "", p.URL)
		}
		return false, err
	}
	return merged.Merged, nil
}
