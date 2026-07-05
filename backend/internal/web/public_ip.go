package web

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"
)

const publicIPCacheTTL = 10 * time.Minute

var defaultPublicIPProviders = []string{
	"https://api.ipify.org",
	"https://checkip.amazonaws.com",
	"https://ifconfig.me/ip",
}

type publicIPResult struct {
	IP        string `json:"ip"`
	CheckedAt string `json:"checkedAt"`
	Source    string `json:"source,omitempty"`
	Cached    bool   `json:"cached"`
}

type publicIPResolver struct {
	client    *http.Client
	providers []string
	ttl       time.Duration

	mu        sync.Mutex
	cached    publicIPResult
	cachedAt  time.Time
	hasCached bool
}

func newPublicIPResolver(providers []string) *publicIPResolver {
	return &publicIPResolver{
		client:    &http.Client{Timeout: 3 * time.Second},
		providers: append([]string(nil), providers...),
		ttl:       publicIPCacheTTL,
	}
}

func (r *publicIPResolver) Resolve(ctx context.Context, force bool) (publicIPResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if !force && r.hasCached && now.Sub(r.cachedAt) < r.ttl {
		result := r.cached
		result.Cached = true
		return result, nil
	}

	var lastErr error
	for _, provider := range r.providers {
		ip, err := r.fetch(ctx, provider)
		if err != nil {
			lastErr = err
			continue
		}
		result := publicIPResult{
			IP:        ip,
			CheckedAt: now.UTC().Format(time.RFC3339),
			Source:    provider,
			Cached:    false,
		}
		r.cached = result
		r.cachedAt = now
		r.hasCached = true
		return result, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no public IP providers configured")
	}
	return publicIPResult{}, lastErr
}

func (r *publicIPResolver) fetch(ctx context.Context, provider string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, provider, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "stardew-anxi-panel/public-ip-check")

	resp, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("%s returned HTTP %d", provider, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 128))
	if err != nil {
		return "", err
	}
	ip := strings.TrimSpace(string(body))
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return "", fmt.Errorf("%s returned invalid IP %q", provider, ip)
	}
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsMulticast() || addr.IsUnspecified() {
		return "", fmt.Errorf("%s returned non-public IP %q", provider, ip)
	}
	return addr.String(), nil
}

// handleInstancePublicIP handles GET /api/instances/:id/public-ip.
func (s *server) handleInstancePublicIP(w http.ResponseWriter, r *http.Request, instanceID string) {
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}
	if _, ok := s.loadInstance(w, r, instanceID); !ok {
		return
	}

	force := r.URL.Query().Get("refresh") == "1" || strings.EqualFold(r.URL.Query().Get("refresh"), "true")
	result, err := s.publicIPResolver.Resolve(r.Context(), force)
	if err != nil {
		s.logger.Warn("public IP check failed", "instance", instanceID, "error", err)
		writeError(w, http.StatusBadGateway, "public_ip_failed", "检测服务器公网 IP 失败，请稍后重试")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
