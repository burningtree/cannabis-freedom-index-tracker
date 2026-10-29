// Package harvest sends discovered URLs directly to durable intake, never to public Nostr.
package harvest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/safeurl"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/store"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Collector struct {
	Store          *store.Store
	Feeds, Queries []string
}

func hash(s string) string { v := sha256.Sum256([]byte(s)); return hex.EncodeToString(v[:]) }
func fetch(ctx context.Context, raw string) ([]byte, error) {
	u, err := safeurl.Normalize(raw)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	res, err := safeurl.Client().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, errors.New("harvest source unavailable")
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, 2*1024*1024+1))
	if err != nil || len(b) > 2*1024*1024 {
		return nil, errors.New("harvest response too large")
	}
	return b, nil
}
func (c Collector) ingest(ctx context.Context, origin, source, raw string) error {
	u, err := safeurl.Normalize(raw)
	if err != nil {
		return nil
	}
	_, err = c.Store.Accept(ctx, store.Intake{Origin: origin, URL: raw, CanonicalURL: u, Provenance: store.JSON(map[string]any{"source": source, "discovered_at": time.Now().Unix()}), IdempotencyKey: origin + ":" + hash(source+"\n"+u)})
	return err
}
func (c Collector) checkpoint(ctx context.Context, key, cursor string) error {
	_, err := c.Store.DB.ExecContext(ctx, "INSERT INTO harvest_state VALUES(?,?,?) ON CONFLICT(provider) DO UPDATE SET cursor=excluded.cursor,updated_at=excluded.updated_at", key, cursor, time.Now().Unix())
	return err
}

type Feed struct {
	Channel struct {
		Items []struct {
			Link string `xml:"link"`
		} `xml:"item"`
	} `xml:"channel"`
	Entries []struct {
		Links []struct {
			Href string `xml:"href,attr"`
			Rel  string `xml:"rel,attr"`
		} `xml:"link"`
	} `xml:"entry"`
}

func ParseFeed(b []byte) ([]string, error) {
	var f Feed
	if err := xml.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	urls := []string{}
	for _, i := range f.Channel.Items {
		urls = append(urls, i.Link)
	}
	for _, e := range f.Entries {
		for _, l := range e.Links {
			if l.Rel == "" || l.Rel == "alternate" {
				urls = append(urls, l.Href)
				break
			}
		}
	}
	if len(urls) > 250 {
		return nil, errors.New("feed exceeds candidate bound")
	}
	return urls, nil
}
func (c Collector) RSS(ctx context.Context, feed string) error {
	b, err := fetch(ctx, feed)
	if err != nil {
		return err
	}
	urls, err := ParseFeed(b)
	if err != nil {
		return err
	}
	for _, u := range urls {
		if err = c.ingest(ctx, "rss", feed, u); err != nil {
			return err
		}
	}
	return c.checkpoint(ctx, "rss:"+hash(feed), time.Now().UTC().Format(time.RFC3339))
}
func (c Collector) GDELT(ctx context.Context, query string) error {
	key := "gdelt:" + hash(query)
	cursor := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Hour)
	var saved string
	if c.Store.DB.QueryRowContext(ctx, "SELECT cursor FROM harvest_state WHERE provider=?", key).Scan(&saved) == nil {
		parsed, err := time.Parse(time.RFC3339, saved)
		if err != nil {
			return err
		}
		cursor = parsed
	}
	end := cursor.Add(time.Hour)
	if end.After(time.Now().UTC().Add(-time.Hour)) {
		return nil
	}
	params := url.Values{"query": {query}, "mode": {"artlist"}, "format": {"json"}, "maxrecords": {"250"}, "sort": {"datedesc"}, "startdatetime": {cursor.Format("20060102150405")}, "enddatetime": {end.Format("20060102150405")}}
	b, err := fetch(ctx, "https://api.gdeltproject.org/api/v2/doc/doc?"+params.Encode())
	if err != nil {
		return err
	}
	var response struct {
		Articles []struct {
			URL string `json:"url"`
		} `json:"articles"`
	}
	if err = json.Unmarshal(b, &response); err != nil {
		return err
	}
	if len(response.Articles) >= 250 {
		return errors.New("GDELT partition saturated: narrow the configured query before advancing")
	}
	for _, a := range response.Articles {
		if err = c.ingest(ctx, "gdelt", query, a.URL); err != nil {
			return err
		}
	}
	return c.checkpoint(ctx, key, end.Format(time.RFC3339))
}
func (c Collector) Run(ctx context.Context) {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			for _, feed := range c.Feeds {
				if feed == "" {
					continue
				}
				run, cancel := context.WithTimeout(ctx, time.Minute)
				err := c.RSS(run, strings.TrimSpace(feed))
				cancel()
				if err != nil {
					slog.Warn("RSS harvest pending", "source_id", hash(feed))
				}
			}
			for _, query := range c.Queries {
				if query == "" {
					continue
				}
				run, cancel := context.WithTimeout(ctx, time.Minute)
				err := c.GDELT(run, strings.TrimSpace(query))
				cancel()
				if err != nil {
					slog.Warn("GDELT harvest pending", "source_id", hash(query))
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Minute):
				}
			}
			timer.Reset(15 * time.Minute)
		}
	}
}
