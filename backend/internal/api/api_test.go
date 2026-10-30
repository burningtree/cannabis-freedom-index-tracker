package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/config"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/store"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func server(t *testing.T) (*Server, http.Handler) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "api.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.DB.Close() })
	a := &Server{Store: s, Config: config.Config{PublicURL: "https://tracker.example", Origins: []string{"https://tracker.example"}, IngestToken: strings.Repeat("a", 32), WebhookSecret: "webhook-secret"}}
	return a, a.Handler()
}
func request(h http.Handler, method, path, body, origin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Origin", origin)
	r.RemoteAddr = "198.51.100.20:1234"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestIntakeReceiptAndBatch(t *testing.T) {
	_, h := server(t)
	w := request(h, "POST", "/v1/submissions", `{"url":"https://news.example/raid?utm_source=x","note":"Public note"}`, "https://tracker.example")
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var v struct {
		ID        string `json:"id"`
		StatusURL string `json:"status_url"`
	}
	json.Unmarshal(w.Body.Bytes(), &v)
	if !idPattern.MatchString(v.ID) || !strings.Contains(v.StatusURL, v.ID) {
		t.Fatal(v)
	}
	for _, path := range []string{"/v1/submissions/" + v.ID, "/v1/submissions?ids=" + v.ID} {
		w = request(h, "GET", path, "", "")
		if w.Code != 200 || !strings.Contains(w.Body.String(), "Public note") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
func TestRejectUnsafeInputsAndCORS(t *testing.T) {
	_, h := server(t)
	for _, body := range []string{`{"url":"http://127.0.0.1/a"}`, `{"url":"https://news.example:8080/a"}`, `{"url":"file:///etc/passwd"}`, `{"url":"https://news.example/a","note":"` + strings.Repeat("x", 501) + `"}`, `{"url":"https://news.example","admin":true}`, `{"url":"https://news.example"} {}`} {
		w := request(h, "POST", "/v1/submissions", body, "")
		if w.Code < 400 {
			t.Fatal(body, w.Code)
		}
	}
	w := request(h, "POST", "/v1/submissions", `{"url":"https://news.example/a"}`, "https://evil.example")
	if w.Code != 403 || w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal(w)
	}
	w = request(h, "OPTIONS", "/v1/submissions", "", "https://tracker.example")
	if w.Code != 204 || w.Header().Get("Access-Control-Allow-Origin") != "https://tracker.example" {
		t.Fatal(w)
	}
}
func TestRateLimitAndBoundedLookup(t *testing.T) {
	_, h := server(t)
	var w *httptest.ResponseRecorder
	for range 21 {
		w = request(h, "POST", "/v1/submissions", `{"url":"https://news.example/a"}`, "")
	}
	if w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatal(w.Code)
	}
	ids := strings.Repeat(strings.Repeat("a", 32)+",", 50) + strings.Repeat("a", 32)
	w = request(h, "GET", "/v1/submissions?ids="+ids, "", "")
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
}
func TestWebhookSignatureAndNoFalsePublication(t *testing.T) {
	a, h := server(t)
	id, err := a.Store.Accept(context.Background(), store.Intake{Origin: "web", URL: "https://news.example/a", CanonicalURL: "https://news.example/a"})
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"action":"closed","pull_request":{"merged":true}}`)
	r := httptest.NewRequest("POST", "/v1/webhooks/github", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	mac := hmac.New(sha256.New, []byte(a.Config.WebhookSecret))
	mac.Write(body)
	r = httptest.NewRequest("POST", "/v1/webhooks/github", bytes.NewReader(body))
	r.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 202 {
		t.Fatal(w.Code)
	}
	v, _ := a.Store.Get(context.Background(), id)
	if v.Status != "queued" {
		t.Fatal("webhook bypassed publication checks")
	}
}
func TestMachineAuth(t *testing.T) {
	_, h := server(t)
	w := request(h, "POST", "/v1/ingest/candidate", `{}`, "")
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
}
func TestForwardedIPRequiresTrustedProxy(t *testing.T) {
	s, _ := server(t)
	r := httptest.NewRequest("GET", "/healthz", nil)
	r.RemoteAddr = "198.51.100.5:1234"
	r.Header.Set("X-Forwarded-For", "1.1.1.1")
	if s.clientIP(r) != "198.51.100.5" {
		t.Fatal("untrusted forwarding accepted")
	}
	s.Config.TrustedProxies = []string{"127.0.0.0/8"}
	r.RemoteAddr = "127.0.0.1:1234"
	if s.clientIP(r) != "1.1.1.1" {
		t.Fatal("trusted proxy address not read")
	}
}
func TestActivityBoundedAndMachineCandidatesExcluded(t *testing.T) {
	a, h := server(t)
	ctx := context.Background()
	for i := 0; i < 27; i++ {
		origin := "web"
		if i == 0 {
			origin = "rss"
		}
		id, err := a.Store.Accept(ctx, store.Intake{Origin: origin, URL: fmt.Sprintf("https://news.example/%d", i), CanonicalURL: fmt.Sprintf("https://news.example/%d", i)})
		if err != nil {
			t.Fatal(err)
		}
		v, _ := a.Store.Get(ctx, id)
		if err = a.Store.Transition(ctx, v.CaseID, "not_relevant", "Not an enforcement event.", "", ""); err != nil {
			t.Fatal(err)
		}
	}
	w := request(h, "GET", "/v1/activity", "", "")
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	var data struct {
		Events []map[string]any `json:"events"`
		Next   string           `json:"next_cursor"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Events) != 25 || data.Next == "" {
		t.Fatal("unbounded or missing cursor")
	}
	w = request(h, "GET", "/v1/activity?cursor="+data.Next, "", "")
	json.Unmarshal(w.Body.Bytes(), &data)
	if len(data.Events) != 1 {
		t.Fatal("machine event leaked or pagination repeated", len(data.Events))
	}
}

func TestRepeatedURLReturnsExistingSubmission(t *testing.T) {
	s, h := server(t)
	var first string
	for _, raw := range []string{"https://news.example/same?utm_source=one", "https://news.example/same#section"} {
		body, _ := json.Marshal(map[string]string{"url": raw})
		w := request(h, "POST", "/v1/submissions", string(body), "https://tracker.example")
		if w.Code != 202 {
			t.Fatal(w.Code, w.Body.String())
		}
		var response struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if first == "" {
			first = response.ID
		} else if response.ID != first {
			t.Fatal("new receipt for repeated URL")
		}
	}
	var count int
	s.Store.DB.QueryRow("SELECT count(*) FROM submissions").Scan(&count)
	if count != 1 {
		t.Fatal(count)
	}
}
