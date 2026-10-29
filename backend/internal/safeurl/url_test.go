package safeurl

import (
	"context"
	"net/http"
	"net/netip"
	"strings"
	"testing"
)

func TestBlockedDestinations(t *testing.T) {
	for _, u := range []string{"http://127.0.0.1/", "http://169.254.169.254/latest", "http://10.0.0.1/", "http://100.64.0.1/", "http://[::1]/", "http://[::ffff:127.0.0.1]/", "https://example.com:444/", "ftp://example.com/", "https://user:secret@example.com/", "http://internal/", "http://test.local/"} {
		if _, err := Normalize(u); err == nil {
			t.Fatalf("accepted %s", u)
		}
	}
	for _, ip := range []string{"127.0.0.1", "::1", "::ffff:127.0.0.1", "169.254.169.254", "10.1.1.1", "192.168.1.1", "100.64.0.1", "2001:db8::1", "64:ff9b::a00:1"} {
		if PublicIP(netip.MustParseAddr(ip)) {
			t.Fatal(ip)
		}
	}
}
func TestNormalizeConservatively(t *testing.T) {
	got, err := Normalize("https://NEWS.example:443/path?b=2&utm_source=x&a=1#section")
	if err != nil || got != "https://news.example/path?a=1&b=2" {
		t.Fatal(got, err)
	}
	got, err = Normalize("http://news.example/a/?id=7")
	if err != nil || got != "http://news.example/a/?id=7" {
		t.Fatal("meaningful query/path lost", got, err)
	}
}
func TestConnectTimeBlocksPrivateDNS(t *testing.T) {
	_, err := Fetch(context.Background(), "http://127.0.0.1.nip.io/")
	if err == nil {
		t.Fatal("private DNS destination fetched")
	}
}

func TestDialAndRedirectPolicyCannotBeSkippedByDNS(t *testing.T) {
	client := Client()
	transport := client.Transport.(*http.Transport)
	if _, err := transport.DialContext(context.Background(), "tcp", "127.0.0.1:80"); err == nil || !strings.Contains(err.Error(), "destination blocked") {
		t.Fatal("literal connect did not enforce IP policy", err)
	}
	req, _ := http.NewRequest("GET", "http://169.254.169.254/latest", nil)
	if err := client.CheckRedirect(req, nil); err == nil {
		t.Fatal("private redirect accepted")
	}
}
