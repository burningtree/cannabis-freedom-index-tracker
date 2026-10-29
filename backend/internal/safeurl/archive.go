package safeurl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var ErrAccessChallenge = errors.New("access challenge")

func AccessChallenge(e Evidence) string {
	if e.AccessBlocked != "" {
		return e.AccessBlocked
	}
	title := ""
	if e.Metadata != nil {
		title = e.Metadata.Title
	}
	if m := titleTag.FindStringSubmatch(e.Content); m != nil {
		title = Text(m[1])
	}
	title = strings.TrimSpace(strings.ToLower(title))
	text := strings.ToLower(e.ReadableText())
	cloudflare := strings.Contains(strings.ToLower(e.Content), "/cdn-cgi/challenge-platform/") || strings.Contains(text, "cloudflare")
	if title == "just a moment..." || title == "just a moment…" || strings.Contains(title, "attention required! | cloudflare") {
		return "Cloudflare"
	}
	if cloudflare && len(text) < 10000 && (strings.Contains(text, "verify you are human") || strings.Contains(text, "performing security verification") || strings.Contains(text, "checking your browser") || strings.Contains(text, "enable javascript and cookies to continue") || strings.Contains(text, "sorry, you have been blocked")) {
		return "Cloudflare"
	}
	if len(text) < 3000 && (title == "access denied" || title == "verify you are human" || title == "security verification") {
		return "Website security verification"
	}
	return ""
}

type ArchiveCapture struct{ URL, Timestamp string }

var replayPath = regexp.MustCompile(`^/web/([0-9]{14})(?:[a-z]+_)?/(https?://.+)$`)

// Validate both the archive host and the original target, not just an API-provided URL.
func ValidateArchive(original, replay string) (ArchiveCapture, error) {
	a, err := url.Parse(replay)
	if err != nil || a.Hostname() != "web.archive.org" || a.User != nil || a.Port() != "" || (a.Scheme != "http" && a.Scheme != "https") {
		return ArchiveCapture{}, errors.New("invalid Wayback snapshot URL")
	}
	m := replayPath.FindStringSubmatch(a.Path)
	if m == nil {
		return ArchiveCapture{}, errors.New("invalid Wayback snapshot path")
	}
	target := m[2]
	if a.RawQuery != "" {
		target += "?" + a.RawQuery
	}
	want, err := Normalize(original)
	if err != nil {
		return ArchiveCapture{}, err
	}
	got, err := Normalize(target)
	if err != nil {
		return ArchiveCapture{}, err
	}
	w, _ := url.Parse(want)
	g, _ := url.Parse(got)
	if strings.TrimPrefix(w.Hostname(), "www.") != strings.TrimPrefix(g.Hostname(), "www.") || w.EscapedPath() != g.EscapedPath() || w.RawQuery != g.RawQuery {
		return ArchiveCapture{}, errors.New("Wayback snapshot is for a different source")
	}
	if _, err = time.Parse("20060102150405", m[1]); err != nil {
		return ArchiveCapture{}, errors.New("invalid Wayback capture timestamp")
	}
	return ArchiveCapture{URL: "https://web.archive.org/web/" + m[1] + "id_/" + target, Timestamp: m[1]}, nil
}

func LookupArchive(ctx context.Context, original string) (ArchiveCapture, error) {
	return lookupArchive(ctx, original, Client())
}
func lookupArchive(parent context.Context, original string, client *http.Client) (ArchiveCapture, error) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	normalized, err := Normalize(original)
	if err != nil {
		return ArchiveCapture{}, err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://archive.org/wayback/available?url="+url.QueryEscape(normalized), nil)
	if err != nil {
		return ArchiveCapture{}, err
	}
	res, err := client.Do(req)
	if err != nil {
		return ArchiveCapture{}, errors.New("Wayback availability lookup failed")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return ArchiveCapture{}, fmt.Errorf("Wayback availability answered HTTP %d", res.StatusCode)
	}
	var data struct {
		Snapshots struct {
			Closest struct {
				Available   bool
				URL, Status string
			}
		} `json:"archived_snapshots"`
	}
	if err = json.NewDecoder(io.LimitReader(res.Body, 256*1024)).Decode(&data); err != nil {
		return ArchiveCapture{}, errors.New("invalid Wayback availability response")
	}
	if !data.Snapshots.Closest.Available || data.Snapshots.Closest.Status != "200" {
		return ArchiveCapture{}, errors.New("no accessible Wayback capture was found")
	}
	return ValidateArchive(normalized, data.Snapshots.Closest.URL)
}

// ResolveSource finishes direct/archived access checks before any reservation or model call.
func ResolveSource(ctx context.Context, raw string, fetch func(context.Context, string) (Evidence, error), lookup func(context.Context, string) (ArchiveCapture, error)) (Evidence, error) {
	e, err := fetch(ctx, raw)
	if ctx.Err() != nil {
		return e, ctx.Err()
	}
	if errors.Is(err, ErrUnsafeDestination) {
		return e, err
	}
	_, checkErr := Check(e)
	if errors.Is(checkErr, ErrAccessChallenge) || err == nil {
		err = checkErr
	}
	if err == nil {
		return e, nil
	}
	if e.URL == "" {
		e.URL = raw
	}
	e.AccessBlocked = AccessChallenge(e)
	originalErr := err
	u, _ := url.Parse(raw)
	if u != nil && u.Hostname() == "web.archive.org" {
		return e, err
	}
	capture, archiveErr := lookup(ctx, raw)
	if archiveErr == nil {
		capture, archiveErr = ValidateArchive(raw, capture.URL)
		if archiveErr == nil {
			archived, fetchErr := fetch(ctx, capture.URL)
			if ctx.Err() != nil {
				return e, ctx.Err()
			}
			if fetchErr == nil {
				_, fetchErr = ValidateArchive(raw, archived.FinalURL)
				if fetchErr == nil {
					_, fetchErr = Check(archived)
				}
			}
			if fetchErr == nil {
				archived.URL = raw
				archived.ArchiveURL = capture.URL
				archived.ArchivedAt = capture.Timestamp
				archived.BrowserWarning = strings.TrimSpace(archived.BrowserWarning + " Live page unavailable; using an Internet Archive capture.")
				return archived, nil
			}
			archiveErr = fetchErr
		}
	}
	if ctx.Err() != nil {
		return e, ctx.Err()
	}
	return e, fmt.Errorf("%w; archived fallback unavailable (%v). No AI research was started", originalErr, archiveErr)
}
