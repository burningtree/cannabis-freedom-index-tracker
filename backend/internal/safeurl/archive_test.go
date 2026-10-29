package safeurl

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestChallengePagesAreRejectedBeforeResearch(t *testing.T) {
	for _, e := range []Evidence{
		page("Just a moment...", strings.Repeat("Performing security verification. Verify you are human. Cloudflare. ", 8)),
		page("Attention Required! | Cloudflare", strings.Repeat("Sorry, you have been blocked. ", 12)),
		{AccessBlocked: "Cloudflare", Content: strings.Repeat("challenge ", 100)},
	} {
		c, err := Check(e)
		if !errors.Is(err, ErrAccessChallenge) || c.AccessBlocked != "Cloudflare" {
			t.Fatal(c, err)
		}
	}
	e := page("Police use Cloudflare to protect their website", strings.Repeat("Police seized cannabis. The official website uses Cloudflare. ", 12))
	if _, err := Check(e); err != nil {
		t.Fatal("legitimate article mistaken for a challenge", err)
	}
}
func TestWaybackSnapshotMustMatchOriginalPublicSource(t *testing.T) {
	original := "https://news.example/story?a=1"
	got, err := ValidateArchive(original, "http://web.archive.org/web/20200102030405/http://www.news.example/story?a=1")
	if err != nil || got.URL != "https://web.archive.org/web/20200102030405id_/http://www.news.example/story?a=1" {
		t.Fatal(got, err)
	}
	for _, bad := range []string{
		"https://evil.example/web/20200102030405/https://news.example/story?a=1",
		"https://web.archive.org/web/20200102030405/https://other.example/story?a=1",
		"https://web.archive.org/web/20200102030405/https://news.example/different?a=1",
		"https://web.archive.org/web/20200102030405/http://127.0.0.1/private",
		"https://web.archive.org/web/20209999030405/https://news.example/story?a=1",
		"https://web.archive.org/web/20200102030405/https://news.example/story?a=2",
	} {
		if _, err := ValidateArchive(original, bad); err == nil {
			t.Fatal("accepted unrelated capture", bad)
		}
	}
}
func TestArchiveFallbackSucceedsOrStopsWithoutUsableArticle(t *testing.T) {
	ctx := context.Background()
	original := "https://news.example/story"
	replay := "https://web.archive.org/web/20200102030405id_/https://news.example/story"
	for _, usable := range []bool{true, false} {
		calls := 0
		fetch := func(_ context.Context, u string) (Evidence, error) {
			calls++
			e := page("Just a moment...", strings.Repeat("Cloudflare performing security verification. ", 10))
			e.URL = u
			e.FinalURL = u
			if u == replay && usable {
				e = page("Police seize cannabis", strings.Repeat("Police seized cannabis at a farm. ", 20))
				e.URL = u
				e.FinalURL = u
			}
			return e, nil
		}
		lookup := func(_ context.Context, u string) (ArchiveCapture, error) {
			if u != original {
				t.Fatal(u)
			}
			return ArchiveCapture{URL: replay}, nil
		}
		e, err := ResolveSource(ctx, original, fetch, lookup)
		if usable {
			if err != nil || e.URL != original || e.ArchiveURL != replay || e.ArchivedAt != "20200102030405" {
				t.Fatal(e, err)
			}
			c, err := Check(e)
			if err != nil || c.Mention == "" || c.ArchiveURL != replay || c.Host != "news.example" {
				t.Fatal(c, err)
			}
		} else if err == nil || !errors.Is(err, ErrAccessChallenge) {
			t.Fatal("challenge was admitted as article", e, err)
		}
		if calls != 2 {
			t.Fatal(calls)
		}
	}
}
func TestArchiveLookupUsesAvailabilityAndValidatesResponse(t *testing.T) {
	client := &http.Client{Transport: fixtureTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "archive.org" || r.URL.Query().Get("url") != "https://news.example/story" {
			t.Fatal(r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"archived_snapshots":{"closest":{"available":true,"status":"200","url":"http://web.archive.org/web/20200102030405/https://news.example/story"}}}`)), Header: http.Header{}, Request: r}, nil
	})}
	got, err := lookupArchive(context.Background(), "https://news.example/story", client)
	if err != nil || got.Timestamp != "20200102030405" {
		t.Fatal(got, err)
	}
}
func TestSuccessfulLiveOrUnsafeSourceDoesNotQueryArchive(t *testing.T) {
	for _, unsafe := range []bool{true, false} {
		fetch := func(context.Context, string) (Evidence, error) {
			if unsafe {
				return Evidence{}, ErrUnsafeDestination
			}
			return page("Article", strings.Repeat("Police seized cannabis. ", 20)), nil
		}
		lookup := func(context.Context, string) (ArchiveCapture, error) {
			t.Fatal("unnecessary or unsafe archive lookup")
			return ArchiveCapture{}, nil
		}
		_, err := ResolveSource(context.Background(), "https://news.example/story", fetch, lookup)
		if unsafe != errors.Is(err, ErrUnsafeDestination) {
			t.Fatal(err)
		}
	}
}
