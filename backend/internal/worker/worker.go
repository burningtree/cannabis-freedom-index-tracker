package worker

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/config"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/nostr"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/research"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/safeurl"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/store"
	"gopkg.in/yaml.v3"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Worker struct {
	Store  *store.Store
	Config config.Config
	Nostr  *nostr.Service
	Health *research.Health
}

func (w *Worker) Run(parent context.Context) {
	ctx, cancelWorkers := context.WithCancel(parent)
	defer cancelWorkers()
	outboxDone := make(chan struct{})
	if w.Nostr == nil {
		close(outboxDone)
	}
	defer func() {
		cancelWorkers()
		<-outboxDone
		if w.Nostr != nil {
			w.Nostr.Wait()
		}
	}()

	if w.Nostr != nil {
		w.Nostr.Listen(ctx)
		go func() { defer close(outboxDone); w.outbox(ctx) }()
	}
	// several requests are worked on at once (WORKER_CONCURRENCY); each loop claims its own job,
	// and a claim is one atomic update, so two loops never take the same one
	loops := max(1, w.Config.WorkerConcurrency)
	var wg sync.WaitGroup
	// job storage: build leftovers and finished requests' workspaces are cleaned up
	wg.Add(1)
	go func() { defer wg.Done(); w.janitor(ctx) }()
	for i := 0; i < loops; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); w.jobs(ctx, "") }()
	}
	// and a lane of its own for committing validated drafts, so a finished draft is committed (and
	// pushed) within seconds instead of waiting for a research run to end
	wg.Add(1)
	go func() { defer wg.Done(); w.jobs(ctx, "publish") }()
	wg.Wait()
}

// jobs claims and processes queued jobs (of one kind when kind is set) one after another until the
// context ends.
func (w *Worker) jobs(ctx context.Context, kind string) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:

			j, err := w.Store.ClaimKind(ctx, "jobs", kind, 2*time.Minute)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				slog.Error("job claim failed")
				continue
			}
			jobCtx, cancel := context.WithTimeout(ctx, w.Config.ResearchTimeout)
			stop := make(chan struct{})
			go w.heartbeat(jobCtx, "jobs", j, stop, cancel)
			err = w.process(jobCtx, j)
			close(stop)
			cancel()
			if ctx.Err() != nil {
				err = store.ErrInterrupted
			}
			if err != nil && j.Attempts >= 8 && !errors.Is(err, store.ErrInterrupted) && !errors.Is(err, store.ErrBudget) && !errors.Is(err, store.ErrConfiguration) {
				_ = w.Store.Transition(ctx, j.Ref, "failed", "Processing exhausted its retry budget. Restart this submission on localhost to try again.", "", "")
			}
			finishCtx, finishCancel := context.WithTimeout(context.Background(), 5*time.Second)
			finishErr := w.Store.Finish(finishCtx, "jobs", j, err)
			finishCancel()
			if finishErr != nil {
				slog.Error("job completion failed", "job", j.ID)
			}

		}
	}
}
func (w *Worker) heartbeat(ctx context.Context, table string, j store.Job, stop <-chan struct{}, cancel context.CancelFunc) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case <-ticker.C:
			if w.Store.Heartbeat(ctx, table, j, 2*time.Minute) != nil {
				cancel()
				return
			}
		}
	}
}
func (w *Worker) outbox(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			j, err := w.Store.Claim(ctx, "outbox", 2*time.Minute)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				slog.Error("outbox claim failed")
				continue
			}
			delivery, cancel := context.WithTimeout(ctx, time.Minute)
			err = w.Nostr.Deliver(delivery, j)
			cancel()
			if finishErr := w.Store.Finish(ctx, "outbox", j, err); finishErr != nil {
				slog.Error("outbox completion failed")
			}
		}
	}
}

type report struct {
	Slug    string `json:"slug"`
	Country string `json:"country"`
	Date    string `json:"date"`
	Title   string `json:"title"`
	Sources []struct {
		URL string `yaml:"url" json:"url"`
	} `json:"sources"`
}

func Coverage(repo string) ([]report, error) {
	out := []report{}
	err := filepath.WalkDir(filepath.Join(repo, "reports"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		parts := strings.SplitN(string(b), "---", 3)
		if len(parts) != 3 {
			return errors.New("invalid report front matter")
		}
		var r report
		if err = yaml.Unmarshal([]byte(parts[1]), &r); err != nil {
			return err
		}
		r.Slug = strings.TrimSuffix(d.Name(), ".md")
		out = append(out, r)
		return nil
	})
	return out, err
}
func (w *Worker) process(ctx context.Context, j store.Job) error {
	if j.Kind == "publish" {
		return w.publish(ctx, j)
	}
	var publicationQueued int
	if err := w.Store.DB.QueryRowContext(ctx, "SELECT count(*) FROM jobs WHERE kind='publish' AND ref=?", j.Ref).Scan(&publicationQueued); err != nil {
		return err
	}
	if publicationQueued > 0 {
		return nil
	}

	var u, status, pr string
	if err := w.Store.DB.QueryRowContext(ctx, "SELECT canonical_url,status,pr_url FROM cases WHERE id=?", j.Ref).Scan(&u, &status, &pr); err != nil {
		return err
	}
	if store.Terminal(status) {
		return nil
	}
	if w.Config.Demo {
		for _, state := range []string{"fetching", "classifying", "researching"} {
			if err := w.Store.Transition(ctx, j.Ref, state, "Demonstration worker: no source research or publication has occurred.", "", ""); err != nil {
				return err
			}
		}
		return w.Store.Transition(ctx, j.Ref, w.uncertainOutcome(), "Demonstration complete. No AI research or publication occurred.", "", "")
	}
	var checkpoint researchCheckpoint
	var saved string
	err := w.Store.DB.QueryRowContext(ctx, "SELECT payload FROM research_checkpoints WHERE case_id=?", j.Ref).Scan(&saved)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var result research.Result
	var initial safeurl.Evidence
	var dir string
	if err == nil {
		if err = json.Unmarshal([]byte(saved), &checkpoint); err != nil {
			return fmt.Errorf("invalid research checkpoint: %w", err)
		}
		result, initial, dir = checkpoint.Result, checkpoint.Initial, checkpoint.Dir
		result.CreatedByModel = checkpoint.Model
		initial.ArtifactDir = checkpoint.ArtifactDir
		// Build in a fresh trusted snapshot: the previous checkout may have been
		// interrupted after writing/moving the draft, or the site may have changed.
		dir, err = (research.Runner{Config: w.Config, Store: w.Store}).Snapshot(ctx, j.Ref)
		if err != nil {
			return err
		}
		if err = research.SaveEvidence(dir, []safeurl.Evidence{initial}); err != nil {
			return err
		}
	} else {
		if !w.Config.Demo && (!w.Config.ResearchEnabled() || (w.Health != nil && !w.Health.Ready())) {
			return store.ErrConfiguration
		}
		reports, err := Coverage(w.Config.RepoPath)
		if err != nil {
			return err
		}
		for _, r := range reports {
			for _, source := range r.Sources {
				known, err := safeurl.Normalize(source.URL)
				if err == nil && known == u {
					return w.Store.Transition(ctx, j.Ref, "duplicate", "This source is already cited in an existing report.", r.Slug, "")
				}
			}
		}
		if err = w.Store.Transition(ctx, j.Ref, "fetching", "Opening the submitted source.", "", ""); err != nil {
			return err
		}
		initial, err = w.loadSource(ctx, j.Ref, u)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		sourceCheck, _ := safeurl.Check(initial)
		if b, e := json.Marshal(sourceCheck); e == nil && (initial.Content != "" || initial.Rendered || initial.BrowserWarning != "") {
			_ = w.Store.SetCaseOutput(ctx, j.Ref, map[string]string{"source_check": string(b)})
		}
		if errors.Is(err, safeurl.ErrUnsafeDestination) {
			return w.Store.Transition(ctx, j.Ref, "insufficient_evidence", "The source destination is not permitted for public research.", "", "")
		}
		// The source check: the submitted page itself must load, be readable and be about cannabis before
		// any AI research. A report rests on the source that was sent, not on whatever else can be found.
		if err != nil {
			return w.Store.Transition(ctx, j.Ref, "source_unavailable", "The submitted page could not be loaded: "+err.Error()+". Send a link that opens without a login or paywall, or another report of the same event.", "", "")
		}
		check, err := safeurl.Check(initial)
		if b, jerr := json.Marshal(check); jerr == nil {
			_ = w.Store.SetCaseOutput(ctx, j.Ref, map[string]string{"source_check": string(b)})
		}
		if err != nil {
			return w.Store.Transition(ctx, j.Ref, "source_unavailable", "The submitted page loaded, but "+err.Error()+". Send a link to a page with the article text.", "", "")
		}
		if check.Mention == "" {
			return w.Store.Transition(ctx, j.Ref, "not_relevant", fmt.Sprintf("The submitted page (%s, %d characters of text) does not mention cannabis.", check.Host, check.Chars), "", "")
		}
		if err = w.Store.Transition(ctx, j.Ref, "classifying", check.Describe(), "", ""); err != nil {
			return err
		}
		run, err := w.Store.Reserve(ctx, j.Ref, "", w.Config.RunCents, w.Config.DailyCents)
		if err != nil {
			return err
		}
		if _, err = w.Store.DB.ExecContext(ctx, "UPDATE research_runs SET model=?,runner=?,status='preparing' WHERE id=?", w.Config.ResearchModel, w.Config.ResearchRunner, run); err != nil {
			return err
		}
		invoked := false
		defer func() {
			if !invoked {
				end, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = w.Store.EndRun(end, run, "failed", "")
				_ = w.Store.AddAIEvent(end, j.Ref, run, "system", "error", "The research workspace could not be prepared. The worker will retry automatically.")
			}
		}()
		runner := research.Runner{Config: w.Config, Store: w.Store}
		dir, err = runner.Snapshot(ctx, j.Ref)
		if err != nil {
			return err
		}
		if err = research.SaveEvidence(dir, []safeurl.Evidence{initial}); err != nil {
			return err
		}
		if err = w.Store.Transition(ctx, j.Ref, "researching", "OpenCode is researching this event and preparing a cited report automatically.", "", ""); err != nil {
			return err
		}
		invoked = true
		result, err = runner.Run(ctx, j.Ref, dir, run, initial, []byte(store.JSON(reports)))
		if err != nil {
			return err
		}
		result.CreatedByModel = w.Config.ResearchModel

		checkpoint = researchCheckpoint{Result: result, Initial: initial, Dir: dir, Model: result.CreatedByModel, ArtifactDir: initial.ArtifactDir}
		if _, err = w.Store.DB.ExecContext(ctx, "INSERT INTO research_checkpoints(case_id,payload,created_at) VALUES(?,?,?) ON CONFLICT(case_id) DO UPDATE SET payload=excluded.payload,created_at=excluded.created_at", j.Ref, store.JSON(checkpoint), time.Now().Unix()); err != nil {
			return err
		}
	}
	// the answer is public: the request page shows it as a report, whatever happens to it next
	_ = w.Store.SetCaseOutput(ctx, j.Ref, map[string]string{"ai_result": store.JSON(result), "draft_path": "", "draft_markdown": ""})
	b, err := os.ReadFile(filepath.Join(w.Config.RepoPath, "src/data/countries.json"))
	if err != nil {
		return err
	}
	var countries []struct {
		ID string `json:"id"`
	}
	if err = json.Unmarshal(b, &countries); err != nil {
		return err
	}
	ids := map[string]bool{}
	for _, c := range countries {
		ids[c.ID] = true
	}
	if err = result.Validate(ids); err != nil {
		return w.Store.Transition(ctx, j.Ref, "failed", "The AI result did not pass validation ("+err.Error()+") and cannot be published automatically.", "", "")
	}

	// a blocked extra source or a missing quotation is noted and worked around; only a really bad
	// answer (nothing confirmed, or most quotations missing) stops here
	if err = w.Store.Transition(ctx, j.Ref, "verifying", "The AI has answered; its sources are being opened again to check every quotation.", "", ""); err != nil {
		return err
	}
	confirmed, evidence, check, err := result.Confirm(ctx, initial, func(ctx context.Context, u string) (safeurl.Evidence, error) { return w.loadSource(ctx, j.Ref, u) })
	if ctx.Err() != nil {
		return ctx.Err()
	}
	_ = w.Store.SetCaseOutput(ctx, j.Ref, map[string]string{"validation": store.JSON(check)})
	if err != nil {
		return w.Store.Transition(ctx, j.Ref, w.uncertainOutcome(), "The facts could not be confirmed in the sources ("+err.Error()+").", "", "")
	}
	result = confirmed
	_ = w.Store.SetCaseOutput(ctx, j.Ref, map[string]string{"ai_result": store.JSON(result)})
	if err = research.SaveEvidence(dir, evidence); err != nil {
		return err
	}
	for _, e := range evidence {
		sum := sha256.Sum256([]byte(e.Content))
		_, err = w.Store.DB.ExecContext(ctx, "INSERT INTO source_urls(url,case_id,final_url,digest,fetched_at) VALUES(?,?,?,?,?) ON CONFLICT(url) DO UPDATE SET final_url=excluded.final_url,digest=excluded.digest,fetched_at=excluded.fetched_at", e.URL, j.Ref, e.FinalURL, hex.EncodeToString(sum[:]), time.Now().Unix())
		if err != nil {
			return err
		}
		var sourceCase string
		if err = w.Store.DB.QueryRowContext(ctx, "SELECT case_id FROM source_urls WHERE url=?", e.URL).Scan(&sourceCase); err != nil {
			return err
		}
		if sourceCase != j.Ref {
			return w.Store.Transition(ctx, j.Ref, w.uncertainOutcome(), "Source overlap with another case could not be resolved automatically.", "", "")
		}
	}
	if result.Outcome != "new_event" && result.Outcome != "existing_case" {
		outcome := result.Outcome
		if outcome == "needs_review" {
			outcome = "insufficient_evidence"
		}
		return w.Store.Transition(ctx, j.Ref, outcome, result.Explanation, "", "")
	}
	if err = w.Store.Transition(ctx, j.Ref, "drafted", "A cited draft is being checked against the repository rules.", "", ""); err != nil {
		return err
	}
	draft, err := research.ValidateBuild(ctx, dir, result)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return w.Store.Transition(ctx, j.Ref, "failed", "The draft did not pass the report build ("+err.Error()+"). Restart to try again.", "", "")
	}
	_ = w.Store.SetCaseOutput(ctx, j.Ref, map[string]string{"draft_path": draft.Path, "draft_markdown": draft.Content})
	digest := sha256.Sum256([]byte(draft.Content))
	payload := draftJob{Dir: dir, Rel: draft.Path, PreviousRel: draft.PreviousPath, Slug: result.Slug, Title: result.Title, Digest: hex.EncodeToString(digest[:])}
	tx, err := w.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = store.Enqueue(tx, "jobs", "publish", j.Ref, store.JSON(payload), time.Now().Unix()); err != nil {
		return err
	}
	return tx.Commit()
}

// Private durable output; never part of the public API. Validation and source checks rerun on recovery.
type researchCheckpoint struct {
	Result                  research.Result
	Initial                 safeurl.Evidence
	Dir, Model, ArtifactDir string
}

type draftJob struct{ Dir, Rel, PreviousRel, Slug, Title, Digest string }

func (w *Worker) publish(ctx context.Context, j store.Job) error {
	var status string
	if err := w.Store.DB.QueryRowContext(ctx, "SELECT status FROM cases WHERE id=?", j.Ref).Scan(&status); err != nil {
		return err
	}
	if status == "committed" {
		return nil
	}
	var d draftJob
	if err := json.Unmarshal([]byte(j.Payload), &d); err != nil {
		return err
	}
	root, err := filepath.Abs(w.Config.Workspaces)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(root, d.Dir)
	if err != nil || strings.HasPrefix(relative, "..") || filepath.IsAbs(relative) {
		return errors.New("invalid workspace")
	}
	b, err := os.ReadFile(filepath.Join(d.Dir, d.Rel))
	if err != nil {
		return err
	}
	sum := sha256.Sum256(b)
	if hex.EncodeToString(sum[:]) != d.Digest {
		return errors.New("validated draft changed")
	}
	if err = w.Store.Transition(ctx, j.Ref, "publishing", "Committing the validated report to the local repository.", d.Slug, ""); err != nil {
		return err
	}
	sha, err := commitReport(ctx, w.Config.RepoPath, j.Ref, d)
	if err != nil {
		return err
	}
	_, err = w.Store.DB.ExecContext(ctx, "INSERT INTO local_publications(case_id,commit_sha,report_path,created_at) VALUES(?,?,?,?) ON CONFLICT(case_id) DO UPDATE SET commit_sha=excluded.commit_sha,report_path=excluded.report_path", j.Ref, sha, d.Rel, time.Now().Unix())
	if err != nil {
		return err
	}
	explanation := "The report was committed to the local repository (" + sha[:12] + ")."
	if w.Config.AutoPush {
		// the commit goes to GitHub straight away, so its link works; a failed push leaves the
		// commit in place and says so
		if err := pushRepo(ctx, w.Config.RepoPath); err != nil {
			explanation += " Pushing it to GitHub failed (" + err.Error() + "); it is in the local repository only until the next push."
		} else {
			explanation += " It was pushed to GitHub."
		}
	} else {
		explanation += " It is available in local development; pushing is separate."
	}
	return w.Store.Transition(ctx, j.Ref, "committed", explanation, d.Slug, "")
}

func (w *Worker) uncertainOutcome() string { return "insufficient_evidence" }
