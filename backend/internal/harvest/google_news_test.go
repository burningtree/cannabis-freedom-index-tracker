package harvest

import (
	"context"
	"encoding/json"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/store"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type newsTransport func(*http.Request) (*http.Response, error)

func (f newsTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func newsResponse(r *http.Request, b string) *http.Response {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(b)), Request: r}
}
func redirectResponse(raw string) string {
	inner, _ := json.Marshal([]string{"garturlres", raw})
	outer, _ := json.Marshal([]any{[]any{"wrb.fr", "Fbv4je", string(inner)}})
	return ")]}'\n\n" + string(outer)
}
func TestGoogleRedirectParsing(t *testing.T) {
	for _, raw := range []string{"http://127.0.0.1/private", "javascript:alert(1)", "https://news.google.com/articles/still-wrapped"} {
		if _, err := parseGoogleRedirect([]byte(redirectResponse(raw))); err == nil {
			t.Fatal("unsafe/unresolved URL accepted", raw)
		}
	}
	raw := "https://publisher.example/story?utm_source=google&edition=uk"
	got, err := parseGoogleRedirect([]byte(redirectResponse(raw)))
	if err != nil || got != "https://publisher.example/story?edition=uk" {
		t.Fatal(got, err)
	}
	if _, err = parseGoogleRedirect([]byte(")]}'\ninvalid")); err == nil {
		t.Fatal("invalid redirect accepted")
	}
}
func TestGoogleScanResolvesAndDurablyDeduplicates(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "news.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	var rpcCalls int
	client := &http.Client{Transport: newsTransport(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Path == "/rss/search":
			if !strings.Contains(r.URL.Query().Get("q"), "cannabis") {
				t.Error("query missing")
			}
			return newsResponse(r, `<rss><channel><item><link>https://news.google.com/rss/articles/first</link></item><item><link>https://news.google.com/rss/articles/second</link></item></channel></rss>`), nil
		case strings.HasPrefix(r.URL.Path, "/rss/articles/"):
			id := strings.TrimPrefix(r.URL.Path, "/rss/articles/")
			return newsResponse(r, `<html><div data-n-a-sg="signature" data-n-a-ts="1793362567" data-n-a-id="`+id+`"></div></html>`), nil
		default:
			rpcCalls++
			if r.Method != "POST" {
				t.Error("redirect request must be POST")
			}
			r.ParseForm()
			if !strings.Contains(r.Form.Get("f.req"), "garturlreq") {
				t.Error("missing redirect payload")
			}
			return newsResponse(r, redirectResponse("https://publisher.example/cannabis-raid")), nil
		}
	})}
	g := GoogleNews{Store: s, Queries: []string{"cannabis raid when:1d"}, MaxArticles: 5, client: client}
	stats, err := g.Scan(context.Background())
	if err != nil || stats.Queued != 1 || stats.Known != 1 {
		t.Fatal(stats, err)
	}
	stats, err = g.Scan(context.Background())
	if err != nil || stats.Queued != 0 || stats.Known != 2 || rpcCalls != 2 {
		t.Fatal("repeated scan re-resolved or requeued candidates", stats, rpcCalls, err)
	}
	var jobs, posts int
	var raw, origin string
	s.DB.QueryRow("SELECT count(*) FROM jobs WHERE kind='research'").Scan(&jobs)
	s.DB.QueryRow("SELECT count(*) FROM outbox").Scan(&posts)
	s.DB.QueryRow("SELECT original_url,origin FROM submissions").Scan(&raw, &origin)
	if jobs != 1 || posts != 0 || origin != "rss" || raw != "https://publisher.example/cannabis-raid" {
		t.Fatal(jobs, posts, raw, origin)
	}
}
func TestGoogleScanDoesNotMarkUnresolvedCandidateSeen(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "news.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	calls := 0
	g := GoogleNews{Store: s, Queries: []string{"cannabis"}, client: &http.Client{Transport: newsTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/rss/search" {
			return newsResponse(r, `<rss><channel><item><link>https://news.google.com/rss/articles/broken</link></item></channel></rss>`), nil
		}
		calls++
		return newsResponse(r, "Google layout changed"), nil
	})}}
	for range 2 {
		stats, err := g.Scan(context.Background())
		if err != nil || stats.Unresolved != 1 || stats.Queued != 0 {
			t.Fatal(stats, err)
		}
	}
	if calls != 2 {
		t.Fatal("unresolved candidate must be retried", calls)
	}
}
func TestGoogleSchedulerRepeatsAndStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	g := GoogleNews{Queries: []string{"cannabis"}, Interval: time.Millisecond, client: &http.Client{Transport: newsTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 2 {
			cancel()
		}
		return newsResponse(r, `<rss><channel/></rss>`), nil
	})}}
	done := make(chan struct{})
	go func() { g.Run(ctx); close(done) }()
	select {
	case <-done:
		if calls != 2 {
			t.Fatal(calls)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("scheduler did not stop")
	}
}
