package harvest

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/safeurl"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/store"
	"html"
	"regexp"
)

const DefaultGoogleNewsQuery = `(cannabis OR marijuana OR hashish) (seizure OR arrest OR raid OR eradication OR sentenced) when:1d`

type GoogleNews struct {
	Store       *store.Store
	Queries     []string
	Interval    time.Duration
	MaxArticles int
	client      *http.Client // tests inject a transport; production always uses safeurl's pinned transport
}

type GoogleNewsStats struct{ Found, Queued, Known, Unresolved int }

func (g GoogleNews) request(ctx context.Context, raw string, form url.Values) ([]byte, string, error) {
	normalized, err := safeurl.Normalize(raw)
	if err != nil {
		return nil, "", err
	}
	method := http.MethodGet
	var body io.Reader
	if form != nil {
		method = http.MethodPost
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, normalized, body)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; CannabisEnforcementTracker discovery)")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	client := g.client
	if client == nil {
		client = safeurl.Client()
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("Google News HTTP %d", res.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, 2*1024*1024+1))
	if err != nil {
		return nil, "", err
	}
	if len(b) > 2*1024*1024 {
		return nil, "", errors.New("Google News response exceeds limit")
	}
	return b, res.Request.URL.String(), nil
}

// Google changes its wrapper format. Fail closed if its public redirect RPC no
// longer resolves, instead of enqueueing an aggregator page as an article.
func (g GoogleNews) resolve(ctx context.Context, raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Hostname() != "news.google.com" {
		return safeurl.Normalize(raw)
	}
	if !strings.HasPrefix(u.Path, "/rss/articles/") && !strings.HasPrefix(u.Path, "/articles/") {
		return "", errors.New("unexpected Google News article path")
	}
	b, final, err := g.request(ctx, raw, nil)
	if err != nil {
		return "", err
	}
	if parsed, _ := url.Parse(final); parsed != nil && parsed.Hostname() != "news.google.com" {
		return safeurl.Normalize(final)
	}
	attrs := map[string]string{}
	tag := regexp.MustCompile(`<[^>]+\bdata-n-a-sg\s*=[^>]+>`).Find(b)
	for _, m := range regexp.MustCompile(`(data-n-a-(?:id|ts|sg))\s*=\s*["']([^"']*)["']`).FindAllSubmatch(tag, -1) {
		attrs[string(m[1])] = html.UnescapeString(string(m[2]))
	}
	ts, err := strconv.ParseInt(attrs["data-n-a-ts"], 10, 64)
	if err != nil {
		return "", errors.New("Google News redirect signature unavailable")
	}
	id := u.Path[strings.LastIndex(u.Path, "/")+1:]
	if attrs["data-n-a-id"] != "" && attrs["data-n-a-id"] != id {
		return "", errors.New("Google News redirect id differs")
	}
	// Public Google News client RPC, observed from its article page; no credentials.
	inner := []any{"garturlreq", []any{[]any{"X", "X", []any{"X", "X"}, nil, nil, 1, 1, "US:en", nil, 1, nil, nil, nil, nil, nil, 0, 1}, "X", "X", 1, []int{1, 1, 1}, 1, 1, nil, 0, 0, nil, 0}, id, ts, attrs["data-n-a-sg"]}
	payload, _ := json.Marshal(inner)
	batch, _ := json.Marshal([]any{[]any{[]any{"Fbv4je", string(payload), nil, "generic"}}})
	b, _, err = g.request(ctx, "https://news.google.com/_/DotsSplashUi/data/batchexecute?rpcids=Fbv4je", url.Values{"f.req": {string(batch)}})
	if err != nil {
		return "", err
	}
	return parseGoogleRedirect(b)
}

func parseGoogleRedirect(b []byte) (string, error) {
	lines := bufio.NewScanner(strings.NewReader(string(b)))
	lines.Buffer(make([]byte, 4096), 2*1024*1024)
	for lines.Scan() {
		line := strings.TrimSpace(lines.Text())
		if !strings.HasPrefix(line, "[") {
			continue
		}
		var rows [][]json.RawMessage
		if json.Unmarshal([]byte(line), &rows) != nil {
			continue
		}
		for _, row := range rows {
			if len(row) < 3 {
				continue
			}
			var method, payload string
			json.Unmarshal(row[1], &method)
			if method != "Fbv4je" {
				continue
			}
			if json.Unmarshal(row[2], &payload) != nil {
				continue
			}
			var result []json.RawMessage
			if json.Unmarshal([]byte(payload), &result) != nil || len(result) < 2 {
				continue
			}
			var kind, raw string
			json.Unmarshal(result[0], &kind)
			json.Unmarshal(result[1], &raw)
			if kind != "garturlres" {
				continue
			}
			normalized, err := safeurl.Normalize(raw)
			if err != nil {
				return "", err
			}
			u, _ := url.Parse(normalized)
			if u.Hostname() == "news.google.com" || strings.HasSuffix(u.Hostname(), ".google.com") {
				return "", errors.New("redirect did not reach a publisher")
			}
			return normalized, nil
		}
	}
	return "", errors.New("Google News publisher URL unavailable")
}

func (g GoogleNews) Scan(ctx context.Context) (stats GoogleNewsStats, err error) {
	max := g.MaxArticles
	if max <= 0 {
		max = 25
	}
	attempted := false
	for _, query := range g.Queries {
		if ctx.Err() != nil {
			return stats, ctx.Err()
		}
		feed := "https://news.google.com/rss/search?" + url.Values{"q": {query}, "hl": {"en-US"}, "gl": {"US"}, "ceid": {"US:en"}}.Encode()
		slog.Info("Google News search started", "query", query)
		b, _, fetchErr := g.request(ctx, feed, nil)
		if fetchErr != nil {
			slog.Warn("Google News search failed", "query", query, "error", fetchErr)
			continue
		}
		links, parseErr := ParseFeed(b)
		if parseErr != nil {
			slog.Warn("Google News feed invalid", "query", query, "error", parseErr)
			continue
		}
		stats.Found += len(links)
		for _, link := range links {
			if stats.Queued >= max {
				break
			}
			key := "google-news:item:" + hash(link)
			var saved string
			dbErr := g.Store.DB.QueryRowContext(ctx, "SELECT cursor FROM harvest_state WHERE provider=?", key).Scan(&saved)
			if dbErr == nil {
				stats.Known++
				slog.Debug("Google News candidate already seen", "candidate", hash(link))
				continue
			}
			if !errors.Is(dbErr, sql.ErrNoRows) {
				return stats, dbErr
			}
			// Pace attempts even when Google returns an error or changes its markup.
			if attempted {
				select {
				case <-ctx.Done():
					return stats, ctx.Err()
				case <-time.After(time.Second):
				}
			}
			attempted = true
			run, cancel := context.WithTimeout(ctx, 30*time.Second)
			publisher, resolveErr := g.resolve(run, link)
			cancel()
			if resolveErr != nil {
				if ctx.Err() != nil {
					return stats, ctx.Err()
				}
				stats.Unresolved++
				slog.Warn("Google News article unresolved; retry next scan", "candidate", hash(link), "error", resolveErr)
				continue
			}
			var exists bool
			if err = g.Store.DB.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM cases WHERE canonical_url=? UNION ALL SELECT 1 FROM source_urls WHERE url=? OR final_url=?)", publisher, publisher, publisher).Scan(&exists); err != nil {
				return stats, err
			}
			if exists {
				stats.Known++
				slog.Debug("Google News publisher already queued or covered", "url", publisher)
			} else {
				id, acceptErr := g.Store.Accept(ctx, store.Intake{Origin: "rss", URL: publisher, CanonicalURL: publisher, IdempotencyKey: "google-news:" + hash(publisher), Provenance: store.JSON(map[string]any{"provider": "google-news", "query": query, "discovery_url": link, "discovered_at": time.Now().Unix()})})
				if acceptErr != nil {
					return stats, acceptErr
				}
				stats.Queued++
				slog.Info("Google News article queued", "url", publisher, "submission_id", id)
			}
			if err = (Collector{Store: g.Store}).checkpoint(ctx, key, publisher); err != nil {
				return stats, err
			}

		}
		slog.Info("Google News search complete", "query", query, "results", len(links))
	}
	return stats, nil
}

func (g GoogleNews) Run(ctx context.Context) {
	interval := g.Interval
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	slog.Info("Google News discovery enabled", "interval", interval.String(), "queries", len(g.Queries), "max_new_articles", g.MaxArticles)
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("Google News discovery stopped")
			return
		case <-timer.C:
			started := time.Now()
			run, cancel := context.WithTimeout(ctx, 5*time.Minute)
			stats, err := g.Scan(run)
			cancel()
			if err != nil && ctx.Err() == nil {
				slog.Warn("Google News scan interrupted", "error", err)
			}
			slog.Info("Google News scan complete", "found", stats.Found, "queued", stats.Queued, "known", stats.Known, "unresolved", stats.Unresolved, "duration", time.Since(started).String(), "next_scan_in", interval.String())
			timer.Reset(interval)
		}
	}
}
