package safeurl

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"sync/atomic"
	"time"
)

// browserProxy is the only way out for the capture browser: an HTTP proxy on the loopback interface,
// protected by a one-capture password. Chromium makes its own connections through it (its own TLS,
// HTTP/2 and headers reach the publisher, as from a reader's browser, so sites behind bot checks
// see a browser and not a Go client), while the proxy decides where they may go: HTTPS (CONNECT) to
// port 443 and plain HTTP to port 80, only of hosts whose every address is public, dialled at the
// checked address so a second DNS answer cannot rebind it. Connections and bytes are capped.
func browserProxy(ctx context.Context) (string, string, func(), error) {
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
	p := &browserProxyHandler{ctx: ctx, auth: "Basic " + base64.StdEncoding.EncodeToString([]byte("capture:"+token)), dial: dialPublic}
	server := &http.Server{Handler: p, ReadHeaderTimeout: 5 * time.Second, MaxHeaderBytes: 16384}
	go server.Serve(listener)
	return "http://" + listener.Addr().String(), token, func() { server.Close() }, nil
}

const (
	proxyMaxConnections = 300
	proxyMaxBytes       = 60 * 1024 * 1024 // both directions, TLS included
)

type browserProxyHandler struct {
	ctx   context.Context
	auth  string
	dial  func(ctx context.Context, network, addr string) (net.Conn, error)
	conns atomic.Int64
	bytes atomic.Int64
}

func (p *browserProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("Proxy-Authorization")), []byte(p.auth)) != 1 {
		// Chromium sends the capture's credentials once asked
		w.Header().Set("Proxy-Authenticate", `Basic realm="capture"`)
		http.Error(w, "proxy authentication required", http.StatusProxyAuthRequired)
		return
	}
	if p.conns.Add(1) > proxyMaxConnections || p.bytes.Load() > proxyMaxBytes {
		http.Error(w, "capture resource limit", http.StatusTooManyRequests)
		return
	}
	if r.Method == http.MethodConnect {
		p.tunnel(w, r)
		return
	}
	p.forward(w, r)
}

// tunnel: HTTPS. The proxy only connects the two ends; the TLS session is Chromium's own.
func (p *browserProxyHandler) tunnel(w http.ResponseWriter, r *http.Request) {
	host, port, err := net.SplitHostPort(r.Host)
	if err != nil || port != "443" || host == "" {
		http.Error(w, "destination blocked", http.StatusForbidden)
		return
	}
	upstream, err := p.dial(p.ctx, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, ErrUnsafeDestination) {
			status = http.StatusForbidden
		}
		http.Error(w, "destination unavailable", status)
		return
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		upstream.Close()
		http.Error(w, "tunnel unsupported", http.StatusInternalServerError)
		return
	}
	client, buffered, err := hijacker.Hijack()
	if err != nil {
		upstream.Close()
		return
	}
	if _, err = client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		client.Close()
		upstream.Close()
		return
	}
	// the capture's deadline ends every tunnel with it
	stop := context.AfterFunc(p.ctx, func() { client.Close(); upstream.Close() })
	defer stop()
	done := make(chan struct{}, 2)
	copyCounted := func(dst io.Writer, src io.Reader) {
		_, _ = io.Copy(dst, &countingReader{r: src, n: &p.bytes, max: proxyMaxBytes})
		done <- struct{}{}
	}
	go copyCounted(upstream, io.MultiReader(io.LimitReader(buffered, int64(buffered.Reader.Buffered())), client))
	go copyCounted(client, upstream)
	<-done
	client.Close()
	upstream.Close()
	<-done
}

// forward: plain HTTP, port 80 only, sent on through the same checked dialer without following
// redirects (Chromium follows them itself, through this proxy again).
func (p *browserProxyHandler) forward(w http.ResponseWriter, r *http.Request) {
	if r.URL.Scheme != "http" || r.URL.Host == "" || (r.URL.Port() != "" && r.URL.Port() != "80") {
		http.Error(w, "destination blocked", http.StatusForbidden)
		return
	}
	if _, err := Normalize(r.URL.String()); err != nil {
		http.Error(w, "destination blocked", http.StatusForbidden)
		return
	}
	transport := &http.Transport{Proxy: nil, DialContext: p.dial, ResponseHeaderTimeout: 15 * time.Second, DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	(&httputil.ReverseProxy{
		Rewrite: func(out *httputil.ProxyRequest) {
			out.Out.URL = r.URL
			out.Out.Host = r.Host
			out.Out.Header.Del("Proxy-Authorization")
			out.Out.Header.Del("Proxy-Connection")
		},
		Transport: transport,
		ModifyResponse: func(res *http.Response) error {
			res.Body = struct {
				io.Reader
				io.Closer
			}{&countingReader{r: res.Body, n: &p.bytes, max: proxyMaxBytes}, res.Body}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			status := http.StatusBadGateway
			if errors.Is(err, ErrUnsafeDestination) {
				status = http.StatusForbidden
			}
			http.Error(w, "destination unavailable", status)
		},
	}).ServeHTTP(w, r)
}

// countingReader adds what it reads to a shared total and stops once the total passes max.
type countingReader struct {
	r   io.Reader
	n   *atomic.Int64
	max int64
}

func (c *countingReader) Read(b []byte) (int, error) {
	if c.n.Load() > c.max {
		return 0, errors.New("capture resource limit")
	}
	n, err := c.r.Read(b)
	c.n.Add(int64(n))
	return n, err
}
