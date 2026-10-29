package harvest

import (
	"context"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/store"
	"path/filepath"
	"testing"
)

func TestRSSAndAtom(t *testing.T) {
	for _, xml := range []string{`<rss><channel><item><link>https://news.example/article</link></item></channel></rss>`, `<feed xmlns="http://www.w3.org/2005/Atom"><entry><link rel="self" href="https://feed.example/id"/><link rel="alternate" href="https://news.example/article"/></entry></feed>`} {
		urls, err := ParseFeed([]byte(xml))
		if err != nil || len(urls) != 1 || urls[0] != "https://news.example/article" {
			t.Fatal(urls, err)
		}
	}
}
func TestCandidatesAreIdempotentAndNotSocialPosts(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "harvest.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	c := Collector{Store: s}
	for range 2 {
		if err = c.ingest(context.Background(), "rss", "source-feed", "https://news.example/article"); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	s.DB.QueryRow("SELECT count(*) FROM submissions").Scan(&count)
	if count != 1 {
		t.Fatal(count)
	}
	s.DB.QueryRow("SELECT count(*) FROM outbox").Scan(&count)
	if count != 0 {
		t.Fatal("candidate leaked to Nostr")
	}
	if err = c.checkpoint(context.Background(), "provider", "cursor"); err != nil {
		t.Fatal(err)
	}
	var cursor string
	s.DB.QueryRow("SELECT cursor FROM harvest_state WHERE provider='provider'").Scan(&cursor)
	if cursor != "cursor" {
		t.Fatal("lost harvest cursor")
	}
}
