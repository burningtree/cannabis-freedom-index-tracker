package safeurl

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// Chromium has a dead proxy as a backstop; Playwright fulfills every permitted
// request through this bridge. The actual connections use the same checked,
// pinned public-IP transport as the HTTP fetcher, including API calls and images.
func browserBridge(ctx context.Context) (string, string, func(), error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", "", nil, err
	}
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		listener.Close()
		return "", "", nil, err
	}
	token := hex.EncodeToString(secret)
	client := Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	handler := &bridgeHandler{ctx: ctx, token: token, client: client}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 35 * time.Second, MaxHeaderBytes: 16384}
	go server.Serve(listener)
	return "http://" + listener.Addr().String(), token, func() { server.Close(); client.CloseIdleConnections() }, nil
}

type bridgeHandler struct {
	ctx      context.Context
	token    string
	client   *http.Client
	requests atomic.Int64
	bytes    atomic.Int64
}
type bridgeRequest struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    []byte            `json:"body"`
}
type bridgeResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    []byte            `json:"body"`
}

func (h *bridgeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" || r.URL.Path != "/fetch" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+h.token)) != 1 {
		http.Error(w, "forbidden", 403)
		return
	}
	if h.requests.Add(1) > 200 || h.bytes.Load() > 30*1024*1024 {
		http.Error(w, "capture resource limit", 429)
		return
	}
	var in bridgeRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 256*1024)).Decode(&in) != nil || len(in.Body) > 128*1024 {
		http.Error(w, "invalid request", 400)
		return
	}
	if _, err := Normalize(in.URL); err != nil {
		http.Error(w, "destination blocked", 403)
		return
	}
	if in.Method != "GET" && in.Method != "HEAD" && in.Method != "POST" && in.Method != "OPTIONS" {
		http.Error(w, "method blocked", 403)
		return
	}
	req, err := http.NewRequestWithContext(h.ctx, in.Method, in.URL, strings.NewReader(string(in.Body)))
	if err != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	for key, value := range in.Headers {
		switch strings.ToLower(key) {
		case "host", "connection", "proxy-authorization", "proxy-connection", "accept-encoding", "content-length", "transfer-encoding", "upgrade":
			continue
		}
		req.Header.Set(key, value)
	}
	res, err := h.client.Do(req)
	if err != nil {
		if errors.Is(err, ErrUnsafeDestination) {
			http.Error(w, "destination blocked", 403)
		} else {
			http.Error(w, "upstream request failed", 502)
		}
		return
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 5*1024*1024+1))
	if err != nil || len(body) > 5*1024*1024 || h.bytes.Add(int64(len(body))) > 30*1024*1024 {
		http.Error(w, "capture resource limit", 413)
		return
	}
	headers := map[string]string{}
	for key, values := range res.Header {
		switch strings.ToLower(key) {
		case "content-length", "transfer-encoding", "connection", "keep-alive", "upgrade":
			continue
		}
		separator := ", "
		if strings.EqualFold(key, "set-cookie") {
			separator = "\n"
		}
		headers[strings.ToLower(key)] = strings.Join(values, separator)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bridgeResponse{res.StatusCode, headers, body})
}
