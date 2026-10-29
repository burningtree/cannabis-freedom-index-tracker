package publisher

import (
	"context"
	"fmt"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/config"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/store"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPublicationRequiresAllConfirmations(t *testing.T) {
	for _, v := range []struct{ merged, pages, live bool }{{false, false, false}, {false, true, true}, {true, false, true}, {true, true, false}} {
		if PublicationReady(v.merged, v.pages, v.live) {
			t.Fatal(v)
		}
	}
	if !PublicationReady(true, true, true) {
		t.Fatal("fully confirmed publication refused")
	}
}

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) {
	res, err := f(r)
	if res != nil && res.Request == nil {
		res.Request = r
	}
	return res, err
}
func response(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}
func TestReconciliationDoesNotPublishBeforeDeploymentAndLivePage(t *testing.T) {
	for _, scenario := range []struct {
		name                string
		merged, pages, live bool
		want                string
	}{{"open", false, false, false, "awaiting_review"}, {"deploy_failed", true, false, true, "deploying"}, {"live_missing", true, true, false, "deploying"}, {"complete", true, true, true, "published"}} {
		t.Run(scenario.name, func(t *testing.T) {
			ctx := context.Background()
			s, err := store.Open(filepath.Join(t.TempDir(), "publish.sqlite"))
			if err != nil {
				t.Fatal(err)
			}
			defer s.DB.Close()
			id, err := s.Accept(ctx, store.Intake{Origin: "web", URL: "https://news.example/a", CanonicalURL: "https://news.example/a"})
			if err != nil {
				t.Fatal(err)
			}
			v, _ := s.Get(ctx, id)
			prURL := "https://github.com/owner/repo/pull/1"
			if err = s.Transition(ctx, v.CaseID, "awaiting_review", "A draft awaits review.", "example-raid", prURL); err != nil {
				t.Fatal(err)
			}
			_, err = s.DB.Exec("INSERT INTO publication(case_id,branch,slug,pr_number,created_at) VALUES(?,?,?,?,?)", v.CaseID, "codex/example", "example-raid", 1, time.Now().Unix())
			if err != nil {
				t.Fatal(err)
			}
			c := &Client{Store: s, Config: config.Config{GitHubRepo: "owner/repo", PublicURL: "https://tracker.example"}, token: "fake", expires: time.Now().Add(time.Hour)}
			c.HTTP = &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
				switch r.URL.Path {
				case "/repos/owner/repo/pulls/1":
					return response(200, fmt.Sprintf(`{"number":1,"html_url":%q,"merged":%t,"state":"open","merge_commit_sha":"merged-sha"}`, prURL, scenario.merged)), nil
				case "/repos/owner/repo/deployments":
					return response(200, `[{"id":2,"sha":"merged-sha","environment":"github-pages"}]`), nil
				case "/repos/owner/repo/deployments/2/statuses":
					state := "failure"
					if scenario.pages {
						state = "success"
					}
					return response(200, fmt.Sprintf(`[{"state":%q}]`, state)), nil
				default:
					t.Fatalf("unexpected path %s", r.URL)
					return nil, nil
				}
			})}
			c.LiveHTTP = &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
				if scenario.live {
					return response(200, `<link rel="canonical" href="https://tracker.example/report/example-raid/">`), nil
				}
				return response(404, "missing"), nil
			})}
			for range 2 {
				if err = c.Reconcile(ctx); err != nil {
					t.Fatal(err)
				}
			}
			v, _ = s.Get(ctx, id)
			if v.Status != scenario.want {
				t.Fatalf("got %s, want %s", v.Status, scenario.want)
			}
		})
	}
}
func TestProposalRetryReusesCommitBranchAndPR(t *testing.T) {
	for _, lostAck := range []bool{false, true} {
		t.Run(fmt.Sprintf("lost_ack_%t", lostAck), func(t *testing.T) {
			ctx := context.Background()
			s, err := store.Open(filepath.Join(t.TempDir(), "retry.sqlite"))
			if err != nil {
				t.Fatal(err)
			}
			defer s.DB.Close()
			id, err := s.Accept(ctx, store.Intake{Origin: "web", URL: "https://news.example/a", CanonicalURL: "https://news.example/a"})
			if err != nil {
				t.Fatal(err)
			}
			v, _ := s.Get(ctx, id)
			dir := t.TempDir()
			for _, args := range [][]string{{"init", "-b", "main", dir}, {"-C", dir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "Baseline"}} {
				if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
					t.Fatalf("git: %s %v", out, err)
				}
			}
			rel := "reports/2026/01/example-raid.md"
			if err = os.MkdirAll(filepath.Dir(filepath.Join(dir, rel)), 0755); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(dir, rel), []byte("A validated fixture draft."), 0644); err != nil {
				t.Fatal(err)
			}
			commits, prAttempts := 0, 0
			branchExists, prExists := false, false
			prJSON := `{"number":9,"html_url":"https://github.com/owner/repo/pull/9"}`
			c := &Client{Store: s, Config: config.Config{GitHubRepo: "owner/repo"}, token: "fake", expires: time.Now().Add(time.Hour)}
			c.HTTP = &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
				path := strings.TrimPrefix(r.URL.Path, "/repos/owner/repo")
				switch {
				case path == "/pulls" && r.Method == "GET":
					if prExists {
						return response(200, "["+prJSON+"]"), nil
					}
					return response(200, "[]"), nil
				case strings.HasPrefix(path, "/git/commits/") && r.Method == "GET":
					return response(200, `{"tree":{"sha":"base-tree"}}`), nil
				case path == "/git/trees":
					return response(201, `{"sha":"new-tree"}`), nil
				case path == "/git/commits":
					commits++
					return response(201, `{"sha":"stable-proposal-commit"}`), nil
				case strings.HasPrefix(path, "/git/ref/heads/"):
					if branchExists {
						return response(200, `{"object":{"sha":"stable-proposal-commit"}}`), nil
					}
					return response(404, `{}`), nil
				case path == "/git/refs":
					branchExists = true
					return response(201, `{}`), nil
				case path == "/pulls" && r.Method == "POST":
					prAttempts++
					if prAttempts == 1 {
						if lostAck {
							prExists = true
							return nil, io.ErrUnexpectedEOF
						}
						return response(503, `{}`), nil
					}
					prExists = true
					return response(201, prJSON), nil
				default:
					t.Fatalf("unexpected GitHub request %s %s", r.Method, path)
					return nil, nil
				}
			})}
			if _, err = c.Propose(ctx, v.CaseID, dir, rel, "", "example-raid", "A report"); err == nil {
				t.Fatal("first outage not surfaced")
			}
			for range 2 {
				url, err := c.Propose(ctx, v.CaseID, dir, rel, "", "example-raid", "A report")
				if err != nil || url != "https://github.com/owner/repo/pull/9" {
					t.Fatal(url, err)
				}
			}
			expectedAttempts := 2
			if lostAck {
				expectedAttempts = 1
			}
			if commits != 1 || prAttempts != expectedAttempts {
				t.Fatal("proposal was duplicated", commits, prAttempts)
			}
		})
	}
}
