package main

import (
	"context"
	"errors"
	n "fiatjaf.com/nostr"
	"flag"
	"fmt"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/api"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/config"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/harvest"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/nostr"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/publisher"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/research"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/store"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/worker"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		slog.Error("service stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	checkAI := flag.Bool("check-ai", false, "Check OpenCode binary and run a short model test, then exit")
	keygen := flag.String("keygen", "", "Write new Nostr service keys to a new private env file, then exit")
	resolve := flag.String("resolve", "", "Resolve a submission after human review, then exit")
	outcome := flag.String("outcome", "", "Human resolution: duplicate, not_relevant, routed_to_cfi, insufficient_evidence or needs_review")
	explanation := flag.String("explanation", "", "Public explanation for a human resolution")
	reportSlug := flag.String("report-slug", "", "Existing report slug for a duplicate resolution")
	backup := flag.String("backup", "", "Write an online SQLite backup to a new file, then exit")
	retry := flag.String("retry", "", "Retry dead jobs/outbox for a submission ID, then exit")
	flag.Parse()
	if *keygen != "" {
		f, err := os.OpenFile(*keygen, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = fmt.Fprintf(f, "NOSTR_GATEWAY_SECRET=%s\nNOSTR_BOT_SECRET=%s\n", n.Generate().Hex(), n.Generate().Hex())
		return err
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	c, err := config.Load()
	if err != nil {
		return err
	}
	if *checkAI {
		check, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		version, err := (research.Runner{Config: c}).Probe(check)
		if err != nil {
			return err
		}
		slog.Info("OpenCode startup check passed", "version", version, "model", c.ResearchModel)
		return nil
	}
	s, err := store.Open(c.Database)
	if err != nil {
		return err
	}
	defer s.DB.Close()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if *resolve != "" {
		switch *outcome {
		case "duplicate", "not_relevant", "routed_to_cfi", "insufficient_evidence", "needs_review":
		default:
			return errors.New("invalid human outcome")
		}
		if len(*explanation) < 10 || len(*explanation) > 1500 {
			return errors.New("provide a short public explanation")
		}
		if *outcome == "duplicate" {
			if !regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`).MatchString(*reportSlug) {
				return errors.New("existing report slug required")
			}
			reports, err := worker.Coverage(c.RepoPath)
			if err != nil {
				return err
			}
			found := false
			for _, r := range reports {
				if r.Slug == *reportSlug {
					found = true
				}
			}
			if !found {
				return errors.New("report does not exist")
			}
		}
		v, err := s.Get(ctx, *resolve)
		if err != nil {
			return err
		}
		if err = s.Transition(ctx, v.CaseID, *outcome, *explanation, *reportSlug, ""); err != nil {
			return err
		}
		_, err = s.DB.ExecContext(ctx, "UPDATE jobs SET status='done',lease_until=0 WHERE ref=? AND status IN ('queued','dead')", v.CaseID)
		return err
	}
	if *backup != "" {
		return s.Backup(ctx, *backup)
	}
	if *retry != "" {
		v, err := s.Get(ctx, *retry)
		if err != nil {
			return err
		}
		tx, err := s.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if _, err = tx.Exec("UPDATE jobs SET status='queued',attempts=0,available_at=?,lease_until=0 WHERE ref=? AND status='dead'", time.Now().Unix(), v.CaseID); err != nil {
			return err
		}
		if _, err = tx.Exec("UPDATE outbox SET status='queued',attempts=0,available_at=?,lease_until=0 WHERE (ref=? OR ref LIKE ?) AND status='dead'", time.Now().Unix(), v.ID, v.ID+":%"); err != nil {
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
		if v.Status == "failed" {
			return s.Transition(ctx, v.CaseID, "queued", "An operator requested another processing attempt.", "", "")
		}
		return nil
	}
	health := research.NewHealth()
	w := worker.Worker{Store: s, Config: c, Health: health}
	checkDone := make(chan struct{})
	if !c.Demo && c.ResearchEnabled() {
		go func() {
			defer close(checkDone)
			(research.Runner{Config: c, Store: s}).Check(ctx, health)
			snapshot := health.Snapshot()
			slog.Info("OpenCode startup check", "status", snapshot.Status, "message", snapshot.Message, "version", snapshot.Version)
		}()
	} else {
		health.Set(research.HealthSnapshot{Status: "unconfigured", Message: "OpenCode research is not configured."})
		close(checkDone)
	}
	if c.BotSecret != "" {
		w.Nostr, err = nostr.New(s, c)
		if err != nil {
			return err
		}
		slog.Info("Nostr enabled", "npub", w.Nostr.Npub())
	}
	if c.GitHubAppID != "" {
		w.Publisher = &publisher.Client{Config: c, Store: s}
	}
	collectorDone := make(chan struct{})
	if len(c.RSSFeeds)+len(c.GDELTQueries) > 0 {
		collector := harvest.Collector{Store: s, Feeds: c.RSSFeeds, Queries: c.GDELTQueries}
		go func() { defer close(collectorDone); collector.Run(ctx) }()
	} else {
		close(collectorDone)
	}
	done := make(chan struct{})
	go func() { defer close(done); w.Run(ctx) }()
	server := &http.Server{BaseContext: func(net.Listener) context.Context { return ctx }, Addr: c.Addr, Handler: (&api.Server{Store: s, Config: c, Health: health}).Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		shutdown, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		_ = server.Shutdown(shutdown)
	}()
	slog.Info("intake listening", "address", c.Addr, "demo", c.Demo, "research_enabled", c.ResearchEnabled())
	err = server.ListenAndServe()
	cancel()
	<-done
	<-collectorDone
	<-checkDone
	<-shutdownDone
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
