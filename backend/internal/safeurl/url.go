// Package safeurl applies the same URL policy at intake, redirects and connect time.
package safeurl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strings"
	"time"
)

var ErrUnsafeDestination = errors.New("destination blocked")

func PublicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if ip.Is6() && !netip.MustParsePrefix("2000::/3").Contains(ip) {
		return false
	}
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, s := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001::/23", "2001:db8::/32", "2002::/16", "64:ff9b::/96"} {
		if netip.MustParsePrefix(s).Contains(ip) {
			return false
		}
	}
	return true
}

func Normalize(raw string) (string, error) {
	if len(raw) > 4096 || strings.ContainsAny(raw, "\r\n\t") {
		return "", errors.New("invalid URL")
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Opaque != "" {
		return "", errors.New("use a public HTTP(S) news URL")
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if !strings.Contains(host, ".") || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return "", errors.New("public hostname required")
	}
	if ip, err := netip.ParseAddr(host); err == nil && !PublicIP(ip) {
		return "", errors.New("private address blocked")
	}
	if p := u.Port(); p != "" && !((u.Scheme == "https" && p == "443") || (u.Scheme == "http" && p == "80")) {
		return "", errors.New("unsupported port")
	}
	u.Host = host
	if strings.Contains(host, ":") {
		u.Host = "[" + host + "]"
	}
	u.Fragment = ""
	if u.Path == "" {
		u.Path = "/"
	}
	q := u.Query()
	for k := range q {
		if strings.HasPrefix(strings.ToLower(k), "utm_") || k == "fbclid" || k == "gclid" {
			q.Del(k)
		}
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func Client() *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	tr := &http.Transport{Proxy: nil, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 15 * time.Second, MaxIdleConns: 16, IdleConnTimeout: 30 * time.Second}
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		if len(ips) == 0 {
			return nil, errors.New("no addresses")
		}
		for _, ip := range ips {
			if !PublicIP(ip) {
				return nil, ErrUnsafeDestination
			}
		}
		// Dial the checked literal address: a second DNS lookup cannot rebind it.
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
	}
	return &http.Client{Transport: tr, Timeout: 30 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("redirect limit")
		}
		_, err := Normalize(req.URL.String())
		if err != nil {
			return ErrUnsafeDestination
		}
		return nil
	}}
}

type Evidence struct {
	URL      string `json:"url"`
	FinalURL string `json:"final_url"`
	Content  string `json:"content"`
}

func Fetch(ctx context.Context, raw string) (Evidence, error) {
	normalized, err := Normalize(raw)
	if err != nil {
		return Evidence{}, err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", normalized, nil)
	if err != nil {
		return Evidence{}, err
	}
	req.Header.Set("User-Agent", "CannabisEnforcementTracker/1.0 (source verification)")
	res, err := Client().Do(req)
	if err != nil {
		if errors.Is(err, ErrUnsafeDestination) {
			return Evidence{}, err
		}
		return Evidence{}, fmt.Errorf("the site could not be reached (%w)", err)
	}
	defer res.Body.Close()
	// the reasons are written for the public request page: say what went wrong, plainly
	if res.StatusCode != 200 {
		return Evidence{}, fmt.Errorf("the site answered HTTP %d %s", res.StatusCode, http.StatusText(res.StatusCode))
	}
	ct := res.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/") && !strings.Contains(ct, "application/xhtml+xml") {
		kind := strings.TrimSpace(strings.SplitN(ct, ";", 2)[0])
		if kind == "" {
			kind = "no content type"
		}
		return Evidence{}, fmt.Errorf("it is not a web page (%s)", kind)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, 2*1024*1024+1))
	if err != nil {
		return Evidence{}, fmt.Errorf("the page could not be read completely (%w)", err)
	}
	if len(b) > 2*1024*1024 {
		return Evidence{}, errors.New("the page is larger than 2 MB")
	}
	return Evidence{normalized, res.Request.URL.String(), string(b)}, nil
}

// StableURLs is useful for deterministic provenance comparisons.
func StableURLs(values []string) []string { sort.Strings(values); return values }
