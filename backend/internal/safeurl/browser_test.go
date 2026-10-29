package safeurl

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fixtureTransport func(*http.Request) (*http.Response, error)

func (f fixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestBrowserBridgeBlocksPrivateAndNonstandardDestinations(t *testing.T) {
	calls := 0
	h := &bridgeHandler{ctx: context.Background(), token: "fixture", client: &http.Client{Transport: fixtureTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/html"}}, Body: io.NopCloser(strings.NewReader("public content")), Request: r}, nil
	})}}
	send := func(target, token string) int {
		body, _ := json.Marshal(bridgeRequest{URL: target, Method: "GET"})
		r := httptest.NewRequest("POST", "/fetch", strings.NewReader(string(body)))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	for _, target := range []string{"http://127.0.0.1/", "http://169.254.169.254/", "http://10.0.0.1/", "http://[::1]/", "file:///etc/passwd", "https://news.example:8443/a", "http://localhost/"} {
		if status := send(target, "fixture"); status != 403 {
			t.Fatal(target, status)
		}
	}
	if status := send("https://news.example/story", "wrong-token"); status != 403 {
		t.Fatal(status)
	}
	if calls != 0 {
		t.Fatal("private requests reached transport")
	}
	if status := send("https://news.example/story", "fixture"); status != 200 || calls != 1 {
		t.Fatal(status, calls)
	}
}
func TestRenderedChecksUseVisibleTextAndDoNotExposeRawStructuredData(t *testing.T) {
	e := Evidence{URL: "https://news.example/a", FinalURL: "https://news.example/a", Rendered: true, Text: strings.Repeat("The public article discusses a cannabis seizure and police arrests. ", 5), Content: "<title>Public source</title><script>Hidden unsupported allegation.</script>", Screenshot: "page.png", Metadata: &Metadata{Title: "Public source", Description: "SEO description", Authors: []string{"Reporter"}, JSONLD: []json.RawMessage{json.RawMessage(`{"articleBody":"private structured-data body"}`)}}}
	check, err := Check(e)
	if err != nil || !check.Rendered || !check.Screenshot || check.Metadata == nil || check.Metadata.Description != "SEO description" {
		t.Fatal(check, err)
	}
	if !e.Supports("a cannabis seizure and police arrests") || e.Supports("Hidden unsupported allegation.") {
		t.Fatal("verification did not use visible text")
	}
	public, _ := json.Marshal(check)
	if strings.Contains(string(public), "private structured-data body") {
		t.Fatal("raw structured data leaked")
	}
	e.Text = strings.Repeat("The visible article is about an unrelated public event. ", 6)
	e.Metadata.Description = "Cannabis only appears in SEO metadata."
	check, err = Check(e)
	if err != nil || check.Mention != "" {
		t.Fatal("SEO descriptions became article evidence", check, err)
	}
}
