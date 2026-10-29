package config

import (
	"errors"
	"net/netip"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr, Database, PublicURL, RepoPath, Workspaces, IngestToken                string
	Origins, Relays, RSSFeeds, GDELTQueries, TrustedProxies                     []string
	GatewaySecret, BotSecret                                                    string
	ResearchImage, ResearchNetwork, ResearchModel, ResearchKey                  string
	ResearchRunner, OpenCodeBinary, OpenCodeAuthFile                            string
	AutoPublish                                                                 bool
	RawAILogs                                                                   bool
	ResearchTimeout                                                             time.Duration
	ResearchMaxSteps                                                            int // model steps a research run may take before it must answer
	RunCents, DailyCents                                                        int
	GitHubAppID, GitHubInstallationID, GitHubKeyFile, GitHubRepo, WebhookSecret string
	Demo                                                                        bool
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func Load() (Config, error) {
	c := Config{Addr: env("HTTP_ADDR", "127.0.0.1:8080"), Database: env("DATABASE_PATH", "data/tracker.sqlite"), PublicURL: strings.TrimRight(env("PUBLIC_BASE_URL", "https://tracker.cannabisfreedom.fyi"), "/"), RepoPath: env("REPO_PATH", ".."), Workspaces: env("WORKSPACE_PATH", "data/jobs"), IngestToken: os.Getenv("INGEST_TOKEN"), Origins: strings.Split(env("CORS_ORIGINS", "https://tracker.cannabisfreedom.fyi"), ","), GatewaySecret: os.Getenv("NOSTR_GATEWAY_SECRET"), BotSecret: os.Getenv("NOSTR_BOT_SECRET"), ResearchImage: os.Getenv("RESEARCH_IMAGE"), ResearchNetwork: os.Getenv("RESEARCH_NETWORK"), ResearchModel: os.Getenv("RESEARCH_MODEL"), ResearchKey: os.Getenv("RESEARCH_API_KEY"), GitHubAppID: os.Getenv("GITHUB_APP_ID"), GitHubInstallationID: os.Getenv("GITHUB_INSTALLATION_ID"), GitHubKeyFile: os.Getenv("GITHUB_APP_PRIVATE_KEY_FILE"), GitHubRepo: env("GITHUB_REPO", "burningtree/cannabis-freedom-index-tracker"), WebhookSecret: os.Getenv("GITHUB_WEBHOOK_SECRET"), Demo: env("DEMO_WORKER", "false") == "true"}
	c.RawAILogs = env("RAW_AI_LOGS", "false") == "true"
	c.ResearchRunner = env("RESEARCH_RUNNER", "docker")
	c.OpenCodeBinary = env("OPENCODE_BIN", "opencode")
	c.OpenCodeAuthFile = os.Getenv("OPENCODE_AUTH_FILE")
	var boolErr error
	c.AutoPublish, boolErr = strconv.ParseBool(env("AUTO_PUBLISH", "true"))
	if boolErr != nil {
		return c, errors.New("invalid AUTO_PUBLISH")
	}
	if c.ResearchRunner != "docker" && c.ResearchRunner != "local" {
		return c, errors.New("RESEARCH_RUNNER must be docker or local")
	}
	if c.ResearchRunner == "local" && c.ResearchModel == "" {
		c.ResearchModel = "opencode-go/deepseek-v4.1-flash"
	}
	if raw := os.Getenv("TRUSTED_PROXY_CIDRS"); raw != "" {
		for _, v := range strings.Split(raw, ",") {
			if _, err := netip.ParsePrefix(v); err != nil {
				return c, errors.New("invalid trusted proxy CIDR")
			}
			c.TrustedProxies = append(c.TrustedProxies, v)
		}
	}
	if raw := os.Getenv("HARVEST_RSS_URLS"); raw != "" {
		c.RSSFeeds = strings.Split(raw, ",")
	}
	if raw := os.Getenv("HARVEST_GDELT_QUERIES"); raw != "" {
		c.GDELTQueries = strings.Split(raw, "|")
	}
	if len(c.RSSFeeds) > 10 || len(c.GDELTQueries) > 10 {
		return c, errors.New("at most 10 configured feeds and queries")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(c.GitHubRepo) {
		return c, errors.New("invalid GitHub repository")
	}
	var err error
	seconds, err := strconv.Atoi(env("RESEARCH_TIMEOUT_SECONDS", "900"))
	if err != nil || seconds < 10 || seconds > 1800 {
		return c, errors.New("invalid research timeout")
	}
	c.ResearchTimeout = time.Duration(seconds) * time.Second
	// with web search working, a run spends steps searching and opening sources; 12 was too few to
	// leave room for the answer
	c.ResearchMaxSteps, err = strconv.Atoi(env("RESEARCH_MAX_STEPS", "24"))
	if err != nil || c.ResearchMaxSteps < 4 || c.ResearchMaxSteps > 100 {
		return c, errors.New("invalid research step limit")
	}
	c.RunCents, err = strconv.Atoi(env("RESEARCH_RUN_CENTS", "100"))
	if err != nil || c.RunCents < 1 {
		return c, errors.New("invalid run budget")
	}
	c.DailyCents, err = strconv.Atoi(env("RESEARCH_DAILY_CENTS", "1000"))
	if err != nil || c.DailyCents < c.RunCents {
		return c, errors.New("invalid daily budget")
	}
	for _, v := range append([]string{c.PublicURL}, c.Origins...) {
		u, e := url.Parse(v)
		if e != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return c, errors.New("invalid public URL or CORS origin")
		}
	}
	if (c.GatewaySecret == "") != (c.BotSecret == "") {
		return c, errors.New("configure both Nostr identities")
	}
	if c.GatewaySecret != "" && c.GatewaySecret == c.BotSecret {
		return c, errors.New("Nostr identities must differ")
	}
	if raw := os.Getenv("NOSTR_RELAYS"); raw != "" {
		for _, v := range strings.Split(raw, ",") {
			u, e := url.Parse(v)
			if e != nil || u.Scheme != "wss" || u.Host == "" || u.User != nil {
				return c, errors.New("invalid relay")
			}
			c.Relays = append(c.Relays, v)
		}
	}
	seenRelays := map[string]bool{}
	for _, relay := range c.Relays {
		seenRelays[relay] = true
	}
	if len(c.Relays) > 10 {
		return c, errors.New("at most 10 relays")
	}
	if c.BotSecret != "" && len(seenRelays) < 2 {
		return c, errors.New("configure at least two relays")
	}
	if c.ResearchRunner == "docker" && c.ResearchImage != "" && (!regexp.MustCompile(`^[A-Za-z0-9._:/-]+@sha256:[a-f0-9]{64}$`).MatchString(c.ResearchImage) || c.ResearchNetwork == "" || c.ResearchModel == "" || c.ResearchKey == "") {
		return c, errors.New("research requires a digest-pinned image, restricted egress network, model and dedicated budget-limited API key")
	}
	if c.Demo && (c.BotSecret != "" || c.ResearchImage != "" || c.ResearchKey != "") {
		return c, errors.New("demo worker must run without Nostr or AI")
	}
	if c.IngestToken != "" && len(c.IngestToken) < 32 {
		return c, errors.New("ingest token must be at least 32 characters")
	}
	configured := 0
	for _, v := range []string{c.GitHubAppID, c.GitHubInstallationID, c.GitHubKeyFile} {
		if v != "" {
			configured++
		}
	}
	if configured != 0 && configured != 3 {
		return c, errors.New("incomplete GitHub App configuration")
	}
	return c, nil
}

func (c Config) ResearchEnabled() bool {
	return c.ResearchModel != "" && (c.ResearchRunner == "local" || (c.ResearchImage != "" && c.ResearchKey != ""))
}
