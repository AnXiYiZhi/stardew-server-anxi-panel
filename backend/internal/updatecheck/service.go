package updatecheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/netdns"
)

const (
	defaultReleasesURL = "https://api.github.com/repos/anxiyizhi/stardew-server-anxi-panel/releases?per_page=20"
	defaultInterval    = 6 * time.Hour
	defaultJitter      = 36 * time.Minute
	requestTimeout     = 15 * time.Second
	maxResponseBytes   = 1 << 20
)

type CheckStatus string

const (
	StatusPending     CheckStatus = "pending"
	StatusChecking    CheckStatus = "checking"
	StatusOK          CheckStatus = "ok"
	StatusError       CheckStatus = "error"
	StatusUnavailable CheckStatus = "unavailable"
)

// Status is the shared API/cache representation of the panel update state.
// CheckedAt deliberately records the last successful check. A failed request
// changes CheckStatus/CheckError but does not erase the last known release.
type Status struct {
	CurrentVersion   string      `json:"currentVersion"`
	CurrentCommit    string      `json:"currentCommit"`
	CurrentBuildDate string      `json:"currentBuildDate"`
	LatestVersion    string      `json:"latestVersion"`
	UpdateAvailable  bool        `json:"updateAvailable"`
	ReleaseURL       string      `json:"releaseUrl"`
	PublishedAt      *time.Time  `json:"publishedAt"`
	CheckedAt        *time.Time  `json:"checkedAt"`
	CheckStatus      CheckStatus `json:"checkStatus"`
	CheckError       string      `json:"checkError"`
}

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type Options struct {
	CurrentVersion string
	Commit         string
	BuildDate      string
	Client         HTTPDoer
	ReleasesURL    string
	Interval       time.Duration
	Jitter         time.Duration
	Now            func() time.Time
	RandomFloat64  func() float64
	Logger         *slog.Logger
}

type Service struct {
	client        HTTPDoer
	releasesURL   string
	interval      time.Duration
	jitter        time.Duration
	now           func() time.Time
	randomFloat64 func() float64
	logger        *slog.Logger

	checkMu sync.Mutex
	mu      sync.RWMutex
	status  Status
}

func New(opts Options) *Service {
	client := opts.Client
	if client == nil {
		client = netdns.NewClient(requestTimeout)
	}
	releasesURL := strings.TrimSpace(opts.ReleasesURL)
	if releasesURL == "" {
		releasesURL = defaultReleasesURL
	}
	interval := opts.Interval
	if interval <= 0 {
		interval = defaultInterval
	}
	jitter := opts.Jitter
	if jitter < 0 {
		jitter = 0
	} else if jitter == 0 && opts.Interval <= 0 {
		jitter = defaultJitter
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	randomFloat64 := opts.RandomFloat64
	if randomFloat64 == nil {
		randomFloat64 = rand.Float64
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	status := Status{
		CurrentVersion:   strings.TrimSpace(opts.CurrentVersion),
		CurrentCommit:    strings.TrimSpace(opts.Commit),
		CurrentBuildDate: strings.TrimSpace(opts.BuildDate),
		CheckStatus:      StatusPending,
	}
	if _, ok := parseSemver(status.CurrentVersion); !ok {
		status.CheckStatus = StatusUnavailable
		status.CheckError = "当前构建未包含有效的正式版本号"
	}

	return &Service{
		client: client, releasesURL: releasesURL, interval: interval, jitter: jitter,
		now: now, randomFloat64: randomFloat64, logger: logger, status: status,
	}
}

func (s *Service) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

func (s *Service) Check(ctx context.Context) Status {
	s.checkMu.Lock()
	defer s.checkMu.Unlock()

	current, ok := parseSemver(s.Status().CurrentVersion)
	if !ok {
		s.mu.Lock()
		s.status.CheckStatus = StatusUnavailable
		s.status.CheckError = "当前构建未包含有效的正式版本号"
		status := s.status
		s.mu.Unlock()
		return status
	}

	s.mu.Lock()
	s.status.CheckStatus = StatusChecking
	s.status.CheckError = ""
	s.mu.Unlock()

	release, err := s.fetchLatestRelease(ctx)
	if err != nil {
		return s.recordFailure(err)
	}
	latest, ok := parseSemver(release.TagName)
	if !ok {
		return s.recordFailure(errors.New("GitHub 最新正式 Release 没有有效的语义化版本号"))
	}

	checkedAt := s.now().UTC()
	publishedAt := release.PublishedAt.UTC()
	s.mu.Lock()
	s.status.LatestVersion = release.TagName
	s.status.UpdateAvailable = compareSemver(current, latest) < 0
	s.status.ReleaseURL = release.HTMLURL
	s.status.PublishedAt = &publishedAt
	s.status.CheckedAt = &checkedAt
	s.status.CheckStatus = StatusOK
	s.status.CheckError = ""
	status := s.status
	s.mu.Unlock()
	return status
}

func (s *Service) recordFailure(err error) Status {
	s.mu.Lock()
	s.status.CheckStatus = StatusError
	s.status.CheckError = err.Error()
	status := s.status
	s.mu.Unlock()
	s.logger.Warn("panel update check failed", "error", err)
	return status
}

type githubRelease struct {
	TagName     string    `json:"tag_name"`
	HTMLURL     string    `json:"html_url"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
}

func (s *Service) fetchLatestRelease(ctx context.Context) (githubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.releasesURL, nil)
	if err != nil {
		return githubRelease{}, fmt.Errorf("创建 GitHub Release 请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "stardew-anxi-panel-update-check")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := s.client.Do(req)
	if err != nil {
		return githubRelease{}, fmt.Errorf("访问 GitHub Release 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return githubRelease{}, fmt.Errorf("GitHub Release 返回 HTTP %d", resp.StatusCode)
	}

	var releases []githubRelease
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes))
	if err := decoder.Decode(&releases); err != nil {
		return githubRelease{}, fmt.Errorf("解析 GitHub Release 响应失败: %w", err)
	}
	for _, release := range releases {
		if release.Draft || release.Prerelease {
			continue
		}
		if _, ok := parseSemver(release.TagName); !ok {
			continue
		}
		return release, nil
	}
	return githubRelease{}, errors.New("GitHub 没有可用的正式 Release")
}

// Run checks immediately after startup and then waits roughly six hours
// between checks. The default +/-36 minute jitter avoids every installation
// hitting GitHub at the same wall-clock time.
func (s *Service) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		s.Check(ctx)
		timer := time.NewTimer(s.nextDelay())
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}
	}
}

func (s *Service) nextDelay() time.Duration {
	if s.jitter <= 0 {
		return s.interval
	}
	offset := time.Duration((s.randomFloat64()*2 - 1) * float64(s.jitter))
	delay := s.interval + offset
	if delay < time.Minute {
		return time.Minute
	}
	return delay
}

type semVersion struct {
	major, minor, patch uint64
	prerelease          []string
}

func parseSemver(raw string) (semVersion, bool) {
	value := strings.TrimSpace(raw)
	if len(value) > 0 && (value[0] == 'v' || value[0] == 'V') {
		value = value[1:]
	}
	if value == "" {
		return semVersion{}, false
	}
	if plus := strings.IndexByte(value, '+'); plus >= 0 {
		if plus == len(value)-1 {
			return semVersion{}, false
		}
		value = value[:plus]
	}
	core := value
	var prerelease []string
	if dash := strings.IndexByte(value, '-'); dash >= 0 {
		core = value[:dash]
		pre := value[dash+1:]
		if pre == "" {
			return semVersion{}, false
		}
		prerelease = strings.Split(pre, ".")
		for _, identifier := range prerelease {
			if identifier == "" || !isSemverIdentifier(identifier) || (isDigits(identifier) && len(identifier) > 1 && identifier[0] == '0') {
				return semVersion{}, false
			}
		}
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return semVersion{}, false
	}
	numbers := make([]uint64, 3)
	for i, part := range parts {
		if part == "" || !isDigits(part) || (len(part) > 1 && part[0] == '0') {
			return semVersion{}, false
		}
		n, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return semVersion{}, false
		}
		numbers[i] = n
	}
	return semVersion{major: numbers[0], minor: numbers[1], patch: numbers[2], prerelease: prerelease}, true
}

func compareSemver(a, b semVersion) int {
	for _, pair := range [][2]uint64{{a.major, b.major}, {a.minor, b.minor}, {a.patch, b.patch}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	if len(a.prerelease) == 0 && len(b.prerelease) == 0 {
		return 0
	}
	if len(a.prerelease) == 0 {
		return 1
	}
	if len(b.prerelease) == 0 {
		return -1
	}
	limit := len(a.prerelease)
	if len(b.prerelease) < limit {
		limit = len(b.prerelease)
	}
	for i := 0; i < limit; i++ {
		ai, bi := a.prerelease[i], b.prerelease[i]
		if ai == bi {
			continue
		}
		an, aNumeric := numericIdentifier(ai)
		bn, bNumeric := numericIdentifier(bi)
		switch {
		case aNumeric && bNumeric:
			if an < bn {
				return -1
			}
			return 1
		case aNumeric:
			return -1
		case bNumeric:
			return 1
		case ai < bi:
			return -1
		default:
			return 1
		}
	}
	if len(a.prerelease) < len(b.prerelease) {
		return -1
	}
	if len(a.prerelease) > len(b.prerelease) {
		return 1
	}
	return 0
}

func numericIdentifier(value string) (uint64, bool) {
	if !isDigits(value) {
		return 0, false
	}
	n, err := strconv.ParseUint(value, 10, 64)
	return n, err == nil
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isSemverIdentifier(value string) bool {
	for _, r := range value {
		if (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '-' {
			continue
		}
		return false
	}
	return true
}
