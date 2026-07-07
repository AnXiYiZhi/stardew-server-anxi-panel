// Package netdns provides HTTP clients whose name resolution falls back to
// public DNS servers when the host/container's configured resolver fails.
//
// Motivation: the panel ships as a Docker container that end users deploy on
// home NAS boxes. Those boxes often have only a single, flaky upstream DNS
// (a consumer router) that intermittently returns SERVFAIL. Go surfaces that
// as "server misbehaving", which breaks every outbound call to public
// services (Nexus Mods, Docker Hub, public-IP probes). Users are not expected
// to fix their router, so the panel resolves defensively itself: it tries the
// system resolver first (so intranet names and custom DNS still work), and
// only on failure queries well-known public DNS servers directly.
package netdns

import (
	"context"
	"net"
	"net/http"
	"time"
)

// publicDNSServers are queried, in order, only after the system resolver
// fails. Chinese resolvers first (primary user base), then global ones.
var publicDNSServers = []string{
	"223.5.5.5:53",    // AliDNS
	"119.29.29.29:53", // DNSPod
	"1.1.1.1:53",      // Cloudflare
	"8.8.8.8:53",      // Google
}

const dnsQueryTimeout = 3 * time.Second

var baseDialer = &net.Dialer{
	Timeout:   10 * time.Second,
	KeepAlive: 30 * time.Second,
}

// ipLookuper is the subset of *net.Resolver the fallback logic needs; it lets
// tests substitute fake resolvers without touching the network.
type ipLookuper interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

// resolverChain is the system resolver followed by one resolver per public
// DNS server. lookupIP walks it until one returns an address.
var resolverChain = buildResolverChain()

func buildResolverChain() []ipLookuper {
	chain := []ipLookuper{net.DefaultResolver}
	for _, server := range publicDNSServers {
		server := server
		chain = append(chain, &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				d := net.Dialer{Timeout: dnsQueryTimeout}
				return d.DialContext(ctx, network, server)
			},
		})
	}
	return chain
}

// lookupIP resolves host through the resolver chain, returning the first
// non-empty result. IPv4 addresses are ordered before IPv6 so a v4-only
// network doesn't waste the dial budget timing out on an unreachable v6 hop.
func lookupIP(ctx context.Context, chain []ipLookuper, host string) ([]net.IPAddr, error) {
	var lastErr error
	for _, r := range chain {
		ips, err := r.LookupIPAddr(ctx, host)
		if err != nil {
			lastErr = err
			continue
		}
		if len(ips) > 0 {
			return orderIPv4First(ips), nil
		}
	}
	if lastErr == nil {
		lastErr = &net.DNSError{Err: "no addresses found", Name: host, IsNotFound: true}
	}
	return nil, lastErr
}

func orderIPv4First(ips []net.IPAddr) []net.IPAddr {
	ordered := make([]net.IPAddr, 0, len(ips))
	for _, ip := range ips {
		if ip.IP.To4() != nil {
			ordered = append(ordered, ip)
		}
	}
	for _, ip := range ips {
		if ip.IP.To4() == nil {
			ordered = append(ordered, ip)
		}
	}
	return ordered
}

// dialContext resolves the host with the fallback chain, then dials each
// candidate IP until one connects. Address literals bypass resolution.
func dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	if net.ParseIP(host) != nil {
		return baseDialer.DialContext(ctx, network, addr)
	}
	ips, err := lookupIP(ctx, resolverChain, host)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, ip := range ips {
		conn, err := baseDialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// NewTransport returns a clone of http.DefaultTransport that resolves names
// through the DNS-fallback dialer.
func NewTransport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.DialContext = dialContext
	return t
}

// NewClient returns an *http.Client with the given timeout that resolves
// names defensively via NewTransport.
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: NewTransport()}
}
