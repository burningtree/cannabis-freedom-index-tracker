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

// GoogleNewsQuery is one search in one Google News edition (ceid, "country:language"). The edition
// decides which language's articles Google returns, so local terms need their local edition.
type GoogleNewsQuery struct{ Edition, Query string }

var googleEdition = regexp.MustCompile(`^[A-Z]{2}:[a-z]{2,3}(-[A-Za-z0-9]{2,4})?$`)

// ParseGoogleNewsQuery reads "[IN:hi] (गांजा OR चरस) when:1d"; without a prefix the US English
// edition is used.
func ParseGoogleNewsQuery(raw string) (GoogleNewsQuery, error) {
	q := GoogleNewsQuery{Edition: "US:en", Query: strings.TrimSpace(raw)}
	if strings.HasPrefix(q.Query, "[") {
		end := strings.Index(q.Query, "]")
		if end < 0 {
			return q, errors.New("unclosed Google News edition")
		}
		q.Edition, q.Query = q.Query[1:end], strings.TrimSpace(q.Query[end+1:])
	}
	if !googleEdition.MatchString(q.Edition) {
		return q, fmt.Errorf("invalid Google News edition %q", q.Edition)
	}
	if q.Query == "" {
		return q, errors.New("empty Google News query")
	}
	return q, nil
}

// Feed parameters for an edition. Google keys the edition on ceid; hl follows its own locale names.
func (q GoogleNewsQuery) params() url.Values {
	country, lang, _ := strings.Cut(q.Edition, ":")
	hl := lang
	switch lang {
	case "en":
		hl = "en-" + country
	case "zh-Hans":
		hl = "zh-CN"
	case "zh-Hant":
		hl = "zh-" + country
	case "pt-419":
		hl = "pt-BR"
	}
	return url.Values{"q": {q.Query}, "hl": {hl}, "gl": {country}, "ceid": {q.Edition}}
}

// DefaultGoogleNewsQueries covers the countries with the most enforcement reports in their own
// languages and local editions, plus other large enforcement languages. Google News has no
// Nepali, Sinhala, Swahili, Khmer, Persian or Pashto edition: those countries are searched in
// English (and Nepali terms in the Hindi edition, best effort).
var DefaultGoogleNewsQueries = []GoogleNewsQuery{
	{"US:en", DefaultGoogleNewsQuery},
	// India: English and the largest regional languages.
	{"IN:en", `(ganja OR charas OR cannabis OR marijuana OR hashish) (seized OR arrested OR NDPS OR destroyed OR sentenced) when:1d`},
	{"IN:hi", `(गांजा OR गांजे OR चरस OR भांग) (जब्त OR बरामद OR गिरफ्तार OR तस्कर OR नष्ट) when:1d`},
	{"IN:bn", `(গাঁজা OR গাঁজাসহ) (উদ্ধার OR জব্দ OR গ্রেপ্তার OR গ্রেফতার OR আটক) when:1d`},
	{"IN:mr", `(गांजा OR चरस) (जप्त OR अटक OR तस्कर) when:1d`},
	{"IN:ta", `(கஞ்சா) (பறிமுதல் OR கைது OR கடத்தல்) when:1d`},
	{"IN:te", `(గంజాయి) (స్వాధీనం OR అరెస్ట్ OR పట్టివేత OR స్మగ్లింగ్) when:1d`},
	{"IN:ml", `(കഞ്ചാവ്) (പിടികൂടി OR പിടിയിൽ OR അറസ്റ്റ് OR പിടിച്ചെടുത്തു) when:1d`},
	// Nepal, Sri Lanka, Bangladesh.
	{"IN:en", `(Nepal OR "Sri Lanka" OR Bangladesh OR Bhutan) (ganja OR cannabis OR hashish OR marijuana) (seized OR arrested OR destroyed OR sentenced) when:1d`},
	{"IN:hi", `(गाँजा OR चरस) (बरामद OR पक्राउ OR नष्ट) when:1d`},
	{"BD:bn", `(গাঁজা OR গাঁজাসহ) (উদ্ধার OR জব্দ OR গ্রেপ্তার OR গ্রেফতার OR আটক) when:1d`},
	// Africa.
	{"KE:en", `(bhang OR marijuana OR cannabis OR kush) (arrested OR seized OR NACADA OR destroyed OR court OR jailed) when:1d`},
	{"NG:en", `("Indian hemp" OR cannabis OR marijuana OR skunk) (NDLEA OR arrested OR seized OR destroyed OR farm) when:1d`},
	{"MA:fr", `(cannabis OR chira OR haschich OR "résine de cannabis" OR kif) (saisie OR saisi OR arrestation OR interpellé OR démantèlement OR condamné) when:1d`},
	{"FR:fr", `(cannabis OR résine OR haschich OR "herbe de cannabis") (saisie OR interpellé OR démantèlement OR condamné OR perquisition) when:1d`},
	// Arabic: Levant (Syria, Lebanon, Palestine), Egypt and North Africa, the Gulf.
	{"LB:ar", `(حشيش OR الحشيش OR "القنب الهندي" OR ماريجوانا) (ضبط OR حجز OR القبض OR توقيف OR إتلاف OR حكم) when:1d`},
	{"EG:ar", `(حشيش OR الحشيش OR "القنب الهندي" OR بانجو) (ضبط OR حجز OR القبض OR توقيف OR إتلاف OR حكم) when:1d`},
	{"SA:ar", `(حشيش OR الحشيش OR "القنب الهندي" OR ماريجوانا) (ضبط OR حجز OR القبض OR توقيف OR إتلاف OR حكم) when:1d`},
	// Southeast and East Asia.
	{"ID:id", `(ganja OR "ladang ganja") (sita OR disita OR ditangkap OR musnahkan OR dimusnahkan OR vonis OR BNN) when:1d`},
	{"PH:en", `(marijuana OR cannabis OR kush) (PDEA OR arrested OR seized OR uprooted OR eradication OR "buy-bust") when:1d`},
	{"TH:th", `(กัญชา) (จับกุม OR ยึด OR ตรวจยึด OR ของกลาง OR บุกค้น) when:1d`},
	{"SG:en", `(cannabis OR marijuana) (CNB OR arrested OR executed OR hanged OR sentenced OR caning) when:1d`},
	{"MY:ms", `(ganja) (rampas OR dirampas OR ditahan OR tumpas OR seludup) when:1d`},
	{"VN:vi", `("cần sa") ("bắt giữ" OR "thu giữ" OR "triệt phá" OR "tử hình" OR "xét xử") when:1d`},
	{"CN:zh-Hans", `(大麻) (查获 OR 抓获 OR 缴获 OR 判处 OR 缉毒) when:1d`},
	{"HK:zh-Hant", `(大麻) (查獲 OR 拘捕 OR 檢獲 OR 判囚 OR 緝毒) when:1d`},
	{"AU:en", `(Fiji OR Samoa OR Tonga OR Kiribati OR Vanuatu OR "Papua New Guinea" OR "Solomon Islands") (marijuana OR cannabis) (arrested OR seized OR uprooted OR court OR sentenced) when:1d`},
	// Europe and the Americas.
	{"CZ:cs", `(konopí OR marihuana OR marihuanu OR pěstírna OR pěstírnu OR hašiš) (policie OR policisté OR zadrželi OR zajistili OR obvinili OR odsoudil OR soud) when:1d`},
	{"ES:es", `(marihuana OR hachís OR cannabis) (incautan OR incautación OR detenidos OR desmantelan OR "Guardia Civil" OR condenado) when:1d`},
	{"RU:ru", `(марихуана OR марихуаны OR конопля OR конопли OR гашиш OR гашиша) (изъяли OR задержали OR задержан OR осужден OR приговор OR плантация) when:1d`},
	{"TR:tr", `(esrar OR kenevir OR skunk OR "Hint keneviri") ("ele geçirildi" OR gözaltı OR operasyon OR tutuklandı) when:1d`},
	{"BR:pt-419", `(maconha OR haxixe OR skunk) (apreensão OR apreendida OR apreendidos OR preso OR presos OR erradicação OR "pés de maconha") when:1d`},
	{"MX:es-419", `(marihuana OR mariguana OR cannabis) (aseguran OR aseguramiento OR decomiso OR detenidos OR erradicación OR plantío) when:1d`},
}

type GoogleNews struct {
	Store       *store.Store
	Queries     []GoogleNewsQuery
	Interval    time.Duration
	MaxArticles int
	// QueriesPerScan rotates through the queries, this many per scan (0 = all). Searches
	// look back a day, so a query that runs every hour or two misses nothing.
	QueriesPerScan int
	client         *http.Client // tests inject a transport; production always uses safeurl's pinned transport
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

func (g GoogleNews) Scan(ctx context.Context) (GoogleNewsStats, error) {
	return g.scan(ctx, g.Queries)
}

// pause paces requests to Google, even when it returns errors or changes its markup.
func pause(ctx context.Context, paced *bool) error {
	if *paced {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	*paced = true
	return nil
}

func (g GoogleNews) scan(ctx context.Context, queries []GoogleNewsQuery) (stats GoogleNewsStats, err error) {
	max := g.MaxArticles
	if max <= 0 {
		max = 25
	}
	paced := false
	results := make([][]string, len(queries))
	for i, query := range queries {
		if err = pause(ctx, &paced); err != nil {
			return stats, err
		}
		feed := "https://news.google.com/rss/search?" + query.params().Encode()
		slog.Info("Google News search started", "edition", query.Edition, "query", query.Query)
		b, _, fetchErr := g.request(ctx, feed, nil)
		if fetchErr != nil {
			slog.Warn("Google News search failed", "edition", query.Edition, "query", query.Query, "error", fetchErr)
			continue
		}
		links, parseErr := ParseFeed(b)
		if parseErr != nil {
			slog.Warn("Google News feed invalid", "edition", query.Edition, "query", query.Query, "error", parseErr)
			continue
		}
		results[i] = links
		stats.Found += len(links)
		slog.Info("Google News search complete", "edition", query.Edition, "query", query.Query, "results", len(links))
	}
	// Take candidates round-robin, so one busy edition cannot use up the whole budget.
	seen := map[string]bool{}
	for rank, more := 0, true; more && stats.Queued < max; rank++ {
		more = false
		for i, query := range queries {
			if rank >= len(results[i]) {
				continue
			}
			more = true
			if stats.Queued >= max {
				break
			}
			link := results[i][rank]
			if seen[link] {
				continue
			}
			seen[link] = true
			if err = g.consider(ctx, query, link, &stats, &paced); err != nil {
				return stats, err
			}
		}
	}
	return stats, nil
}

func (g GoogleNews) consider(ctx context.Context, query GoogleNewsQuery, link string, stats *GoogleNewsStats, paced *bool) (err error) {
	key := "google-news:item:" + hash(link)
	var saved string
	dbErr := g.Store.DB.QueryRowContext(ctx, "SELECT cursor FROM harvest_state WHERE provider=?", key).Scan(&saved)
	if dbErr == nil {
		stats.Known++
		slog.Debug("Google News candidate already seen", "candidate", hash(link))
		return nil
	}
	if !errors.Is(dbErr, sql.ErrNoRows) {
		return dbErr
	}
	if err = pause(ctx, paced); err != nil {
		return err
	}
	run, cancel := context.WithTimeout(ctx, 30*time.Second)
	publisher, resolveErr := g.resolve(run, link)
	cancel()
	if resolveErr != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		stats.Unresolved++
		slog.Warn("Google News article unresolved; retry next scan", "candidate", hash(link), "error", resolveErr)
		return nil
	}
	var exists bool
	if err = g.Store.DB.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM cases WHERE canonical_url=? UNION ALL SELECT 1 FROM source_urls WHERE url=? OR final_url=?)", publisher, publisher, publisher).Scan(&exists); err != nil {
		return err
	}
	if exists {
		stats.Known++
		slog.Debug("Google News publisher already queued or covered", "url", publisher)
	} else {
		id, acceptErr := g.Store.Accept(ctx, store.Intake{Origin: "rss", URL: publisher, CanonicalURL: publisher, IdempotencyKey: "google-news:" + hash(publisher), Provenance: store.JSON(map[string]any{"provider": "google-news", "edition": query.Edition, "query": query.Query, "discovery_url": link, "discovered_at": time.Now().Unix()})})
		if acceptErr != nil {
			return acceptErr
		}
		stats.Queued++
		slog.Info("Google News article queued", "url", publisher, "edition", query.Edition, "submission_id", id)
	}
	return (Collector{Store: g.Store}).checkpoint(ctx, key, publisher)
}

// batch returns the queries for scan n, rotating QueriesPerScan at a time.
func (g GoogleNews) batch(n int) []GoogleNewsQuery {
	size := g.QueriesPerScan
	if size <= 0 || size >= len(g.Queries) {
		return g.Queries
	}
	out := make([]GoogleNewsQuery, 0, size)
	for i := range size {
		out = append(out, g.Queries[(n*size+i)%len(g.Queries)])
	}
	return out
}

func (g GoogleNews) Run(ctx context.Context) {
	interval := g.Interval
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	slog.Info("Google News discovery enabled", "interval", interval.String(), "queries", len(g.Queries), "queries_per_scan", len(g.batch(0)), "max_new_articles", g.MaxArticles)
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for n := 0; ; n++ {
		select {
		case <-ctx.Done():
			slog.Info("Google News discovery stopped")
			return
		case <-timer.C:
			started := time.Now()
			run, cancel := context.WithTimeout(ctx, 5*time.Minute)
			stats, err := g.scan(run, g.batch(n))
			cancel()
			if err != nil && ctx.Err() == nil {
				slog.Warn("Google News scan interrupted", "error", err)
			}
			slog.Info("Google News scan complete", "found", stats.Found, "queued", stats.Queued, "known", stats.Known, "unresolved", stats.Unresolved, "duration", time.Since(started).String(), "next_scan_in", interval.String())
			timer.Reset(interval)
		}
	}
}
