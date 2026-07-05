package docker

import (
	"errors"
	"fmt"
	"log/slog"
	"time"
)

const (
	DefaultDockerPath     = "docker"
	DefaultMaxOutputBytes = 1 << 20
	DefaultLogTail        = 100
	MaxLogTail            = 1000
)

var (
	ErrInvalidWorkDir = errors.New("invalid work dir")
	ErrCommandFailed  = errors.New("docker command failed")
	ErrCommandTimeout = errors.New("docker command timed out")
	ErrInvalidService = errors.New("invalid compose service")
	ErrInvalidTail    = errors.New("invalid log tail")
)

type Options struct {
	DockerPath     string
	Logger         *slog.Logger
	MaxOutputBytes int64
	Timeouts       Timeouts
}

type Timeouts struct {
	Version time.Duration
	Ps      time.Duration
	Logs    time.Duration
	Stats   time.Duration
	Pull    time.Duration
	Up      time.Duration
	Down    time.Duration
	Restart time.Duration
}

type Client struct {
	dockerPath     string
	logger         *slog.Logger
	maxOutputBytes int64
	timeouts       Timeouts
}

type CommandResult struct {
	WorkDir         string   `json:"workDir"`
	Args            []string `json:"args,omitempty"`
	Stdout          string   `json:"stdout"`
	Stderr          string   `json:"stderr"`
	ExitCode        int      `json:"exitCode"`
	DurationMS      int64    `json:"durationMs"`
	TimedOut        bool     `json:"timedOut"`
	StdoutTruncated bool     `json:"stdoutTruncated,omitempty"`
	StderrTruncated bool     `json:"stderrTruncated,omitempty"`
}

type CommandError struct {
	Op     string
	Result CommandResult
	Err    error
}

func (e CommandError) Error() string {
	if e.Err == nil {
		return e.Op
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e CommandError) Unwrap() error {
	return e.Err
}

type LogsOptions struct {
	Service string
	Tail    int
}

type ComposePsResult struct {
	Result   CommandResult    `json:"result"`
	Services []ComposeService `json:"services"`
}

type ComposeService struct {
	Name     string `json:"name,omitempty"`
	Service  string `json:"service,omitempty"`
	State    string `json:"state,omitempty"`
	Status   string `json:"status,omitempty"`
	Health   string `json:"health,omitempty"`
	ExitCode int    `json:"exitCode,omitempty"`
}

type ComposeStatsResult struct {
	Result   CommandResult         `json:"result"`
	Services []ComposeServiceStats `json:"services"`
}

type ComposeServiceStats struct {
	Name             string  `json:"name,omitempty"`
	Container        string  `json:"container,omitempty"`
	ID               string  `json:"id,omitempty"`
	Service          string  `json:"service,omitempty"`
	CPUPerc          float64 `json:"cpuPercent"`
	MemPerc          float64 `json:"memoryPercent"`
	MemUsage         string  `json:"memoryUsage,omitempty"`
	MemUsedBytes     int64   `json:"memoryUsedBytes,omitempty"`
	MemLimitBytes    int64   `json:"memoryLimitBytes,omitempty"`
	NetIO            string  `json:"netIO,omitempty"`
	BlockIO          string  `json:"blockIO,omitempty"`
	PIDs             string  `json:"pids,omitempty"`
	RawCPUPerc       string  `json:"rawCpuPercent,omitempty"`
	RawMemPerc       string  `json:"rawMemoryPercent,omitempty"`
	RawMemUsedBytes  string  `json:"rawMemoryUsedBytes,omitempty"`
	RawMemLimitBytes string  `json:"rawMemoryLimitBytes,omitempty"`
}

func NewClient(options Options) *Client {
	dockerPath := options.DockerPath
	if dockerPath == "" {
		dockerPath = DefaultDockerPath
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	maxOutputBytes := options.MaxOutputBytes
	if maxOutputBytes <= 0 {
		maxOutputBytes = DefaultMaxOutputBytes
	}
	return &Client{
		dockerPath:     dockerPath,
		logger:         logger,
		maxOutputBytes: maxOutputBytes,
		timeouts:       withDefaultTimeouts(options.Timeouts),
	}
}

func withDefaultTimeouts(timeouts Timeouts) Timeouts {
	if timeouts.Version <= 0 {
		timeouts.Version = 10 * time.Second
	}
	if timeouts.Ps <= 0 {
		timeouts.Ps = 15 * time.Second
	}
	if timeouts.Logs <= 0 {
		timeouts.Logs = 20 * time.Second
	}
	if timeouts.Stats <= 0 {
		timeouts.Stats = 12 * time.Second
	}
	if timeouts.Pull <= 0 {
		timeouts.Pull = 30 * time.Minute
	}
	if timeouts.Up <= 0 {
		timeouts.Up = 2 * time.Minute
	}
	if timeouts.Down <= 0 {
		timeouts.Down = 2 * time.Minute
	}
	if timeouts.Restart <= 0 {
		timeouts.Restart = 2 * time.Minute
	}
	return timeouts
}
