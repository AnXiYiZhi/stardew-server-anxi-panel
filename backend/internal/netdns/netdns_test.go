package netdns

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fakeResolver returns fixed IPs (or an error) and records whether it was hit,
// so tests can assert the fallback chain stops at the first success.
type fakeResolver struct {
	ips    []net.IPAddr
	err    error
	called bool
}

func (f *fakeResolver) LookupIPAddr(_ context.Context, _ string) ([]net.IPAddr, error) {
	f.called = true
	return f.ips, f.err
}

func ipAddrs(addrs ...string) []net.IPAddr {
	out := make([]net.IPAddr, 0, len(addrs))
	for _, a := range addrs {
		out = append(out, net.IPAddr{IP: net.ParseIP(a)})
	}
	return out
}

func TestLookupIPFallsBackPastFailures(t *testing.T) {
	good := &fakeResolver{ips: ipAddrs("203.0.113.7")}
	chain := []ipLookuper{
		&fakeResolver{err: errors.New("server misbehaving")}, // system resolver flaked
		&fakeResolver{ips: nil},                              // empty answer, keep going
		good,
	}
	ips, err := lookupIP(context.Background(), chain, "example.com")
	if err != nil {
		t.Fatalf("expected success via fallback, got %v", err)
	}
	if len(ips) != 1 || ips[0].IP.String() != "203.0.113.7" {
		t.Fatalf("unexpected ips: %v", ips)
	}
	if !good.called {
		t.Fatal("fallback resolver was never consulted")
	}
}

func TestLookupIPShortCircuitsOnFirstSuccess(t *testing.T) {
	later := &fakeResolver{ips: ipAddrs("198.51.100.1")}
	chain := []ipLookuper{&fakeResolver{ips: ipAddrs("203.0.113.7")}, later}
	if _, err := lookupIP(context.Background(), chain, "example.com"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if later.called {
		t.Fatal("later resolver should not be consulted after a success")
	}
}

func TestLookupIPAllFail(t *testing.T) {
	chain := []ipLookuper{
		&fakeResolver{err: errors.New("boom")},
		&fakeResolver{err: errors.New("boom")},
	}
	if _, err := lookupIP(context.Background(), chain, "example.com"); err == nil {
		t.Fatal("expected error when every resolver fails")
	}
}

func TestOrderIPv4First(t *testing.T) {
	in := []net.IPAddr{
		{IP: net.ParseIP("2001:db8::1")},
		{IP: net.ParseIP("192.0.2.1")},
	}
	out := orderIPv4First(in)
	if out[0].IP.To4() == nil {
		t.Fatalf("expected IPv4 first, got %v", out)
	}
}

// TestDialContextResolvesViaChain drives the full dial path: a fake resolver
// maps a bogus hostname to the loopback address of a live listener.
func TestDialContextResolvesViaChain(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		if c, err := ln.Accept(); err == nil {
			_ = c.Close()
		}
	}()

	orig := resolverChain
	resolverChain = []ipLookuper{&fakeResolver{ips: ipAddrs("127.0.0.1")}}
	defer func() { resolverChain = orig }()

	_, port, _ := net.SplitHostPort(ln.Addr().String())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, err := dialContext(ctx, "tcp", net.JoinHostPort("nonexistent.invalid", port))
	if err != nil {
		t.Fatalf("dialContext: %v", err)
	}
	_ = conn.Close()
}

// TestNewClientUsesFallbackDialer confirms NewClient's transport routes
// through the chain-based dialer (the resolver, not the OS, decides the IP).
func TestNewClientUsesFallbackDialer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	host, port, _ := net.SplitHostPort(srv.Listener.Addr().String())

	orig := resolverChain
	resolverChain = []ipLookuper{&fakeResolver{ips: ipAddrs(host)}}
	defer func() { resolverChain = orig }()

	client := NewClient(3 * time.Second)
	resp, err := client.Get("http://fake-host.invalid:" + port + "/")
	if err != nil {
		t.Fatalf("request via fallback dialer failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}
}
