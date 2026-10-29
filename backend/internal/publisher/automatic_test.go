package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/config"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/store"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAutomaticMergeRequiresExactValidatedHeadAndPassingChecks(t *testing.T) {
	for _, scenario := range []struct {
		name, head, conclusion string
		pending                bool
		wantMerges             int
		wantState              string
	}{{"success", "validated-sha", "success", false, 1, "publishing"}, {"failed_check", "validated-sha", "failure", false, 0, "failed"}, {"pending", "validated-sha", "", true, 0, "publishing"}, {"changed_head", "unexpected-sha", "success", false, 0, "failed"}} {
		t.Run(scenario.name, func(t *testing.T) {
			ctx := context.Background()
			s, err := store.Open(filepath.Join(t.TempDir(), "automatic.sqlite"))
			if err != nil {
				t.Fatal(err)
			}
			defer s.DB.Close()
			id, err := s.Accept(ctx, store.Intake{Origin: "web", URL: "https://news.example/a", CanonicalURL: "https://news.example/a"})
			if err != nil {
				t.Fatal(err)
			}
			v, _ := s.Get(ctx, id)
			s.Transition(ctx, v.CaseID, "publishing", "Automatic checks in progress.", "example", "https://github.com/owner/repo/pull/1")
			_, err = s.DB.Exec("INSERT INTO publication(case_id,branch,slug,pr_number,head_sha,created_at) VALUES(?,?,?,?,?,?)", v.CaseID, "codex/test", "example", 1, "validated-sha", time.Now().Unix())
			if err != nil {
				t.Fatal(err)
			}
			merges := 0
			c := Client{Store: s, Config: config.Config{AutoPublish: true, GitHubRepo: "owner/repo"}, token: "fake", expires: time.Now().Add(time.Hour)}
			c.HTTP = &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
				path := r.URL.Path
				switch {
				case strings.HasSuffix(path, "/check-runs"):
					status := "completed"
					if scenario.pending {
						status = "in_progress"
					}
					return response(200, fmt.Sprintf(`{"total_count":1,"check_runs":[{"name":"Validate report","status":%q,"conclusion":%q}]}`, status, scenario.conclusion)), nil
				case strings.HasSuffix(path, "/status"):
					return response(200, `{"total_count":0,"state":"pending"}`), nil
				case strings.HasSuffix(path, "/merge"):
					if r.Method != "PUT" {
						t.Fatal(r.Method)
					}
					var payload map[string]string
					json.NewDecoder(r.Body).Decode(&payload)
					if payload["sha"] != "validated-sha" {
						t.Fatal("merge not fenced to validated SHA")
					}
					merges++
					return response(200, `{"merged":true}`), nil
				default:
					t.Fatal(path)
					return nil, nil
				}
			})}
			p := pull{Number: 1, URL: "https://github.com/owner/repo/pull/1"}
			p.Head.SHA = scenario.head
			p.Base.Ref = "main"
			merged, err := c.automaticMerge(ctx, v.CaseID, p)
			if err != nil {
				t.Fatal(err)
			}
			v, _ = s.Get(ctx, id)
			if merges != scenario.wantMerges || v.Status != scenario.wantState || merged != (scenario.wantMerges == 1) {
				t.Fatal(merges, v.Status, merged)
			}
		})
	}
}
