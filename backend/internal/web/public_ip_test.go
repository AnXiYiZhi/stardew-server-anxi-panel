package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPublicIPResolverFallsBackAndCaches(t *testing.T) {
	calls := 0
	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte("not-an-ip"))
	}))
	defer badServer.Close()

	goodServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte("8.8.8.8\n"))
	}))
	defer goodServer.Close()

	resolver := newPublicIPResolver([]string{badServer.URL, goodServer.URL})
	resolver.ttl = time.Hour

	first, err := resolver.Resolve(context.Background(), false)
	if err != nil {
		t.Fatalf("resolve public IP: %v", err)
	}
	if first.IP != "8.8.8.8" || first.Cached {
		t.Fatalf("first result = %+v, want fresh 8.8.8.8", first)
	}
	if calls != 2 {
		t.Fatalf("calls after first resolve = %d, want 2", calls)
	}

	second, err := resolver.Resolve(context.Background(), false)
	if err != nil {
		t.Fatalf("resolve cached public IP: %v", err)
	}
	if second.IP != "8.8.8.8" || !second.Cached {
		t.Fatalf("second result = %+v, want cached 8.8.8.8", second)
	}
	if calls != 2 {
		t.Fatalf("calls after cached resolve = %d, want 2", calls)
	}
}

func TestPublicIPEndpointRequiresAuthAndReturnsIP(t *testing.T) {
	ipServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("1.1.1.1"))
	}))
	defer ipServer.Close()

	oldProviders := defaultPublicIPProviders
	defaultPublicIPProviders = []string{ipServer.URL}
	t.Cleanup(func() {
		defaultPublicIPProviders = oldProviders
	})

	handler, closeStore := newTestHandler(t)
	defer closeStore()

	_, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username":        "admin",
		"password":        "secret123",
		"confirmPassword": "secret123",
	}, nil)

	unauthorized, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/public-ip", nil, nil)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want 401", unauthorized.Code)
	}

	response, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/public-ip", nil, adminCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("public IP status = %d: %s", response.Code, response.Body.String())
	}
	var payload publicIPResult
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.IP != "1.1.1.1" || payload.CheckedAt == "" || payload.Source == "" {
		t.Fatalf("unexpected public IP payload: %+v", payload)
	}
}
