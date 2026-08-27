package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	restrictedImageRefPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*:[A-Za-z0-9][A-Za-z0-9._-]*$`)
	composeProjectPattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	dockerVolumePattern       = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
	runtimeDigestPattern      = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

const disabledSteamAuthImage = "localhost/anxi-disabled-steam-auth:ignored"

type RuntimeImageMetadata struct {
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

type RuntimeComposeConfig struct {
	Project            string            `json:"project"`
	Services           []string          `json:"services"`
	Images             map[string]string `json:"images"`
	SteamSessionVolume string            `json:"steamSessionVolume"`
}

type RuntimeVolumeMetadata struct {
	Name       string `json:"name"`
	Mountpoint string `json:"mountpoint,omitempty"`
}

// RuntimeImageInspect returns only the immutable image ID/digest fields needed
// by Junimo update preflight. Raw inspect JSON is never returned to callers.
func (c *Client) RuntimeImageInspect(ctx context.Context, dir, imageRef string) (RuntimeImageMetadata, error) {
	if err := validateRestrictedImageRef(imageRef); err != nil {
		return RuntimeImageMetadata{}, err
	}
	// Ask Docker to emit only the two immutable fields we need. Parsing the full
	// inspect document after generic log redaction is unsafe: image environment
	// entries such as VNC_PASSWORD= can be redacted into invalid JSON.
	result, err := c.run(ctx, "docker image inspect", dir, c.timeouts.Ps,
		"image", "inspect", "--format", `{{json .Id}}|{{json .RepoDigests}}`, imageRef)
	if err != nil {
		return RuntimeImageMetadata{}, err
	}
	return parseRuntimeImageInspectOutput(result.Stdout)
}

func parseRuntimeImageInspectOutput(output string) (RuntimeImageMetadata, error) {
	parts := strings.SplitN(strings.TrimSpace(output), "|", 2)
	if len(parts) != 2 {
		return RuntimeImageMetadata{}, errors.New("invalid docker image inspect response")
	}
	var id string
	var repoDigests []string
	if json.Unmarshal([]byte(parts[0]), &id) != nil || json.Unmarshal([]byte(parts[1]), &repoDigests) != nil || !runtimeDigestPattern.MatchString(strings.TrimSpace(id)) {
		return RuntimeImageMetadata{}, errors.New("invalid docker image inspect response")
	}
	digest := ""
	for _, ref := range repoDigests {
		if index := strings.LastIndex(ref, "@sha256:"); index >= 0 {
			candidate := ref[index+1:]
			if runtimeDigestPattern.MatchString(candidate) {
				digest = candidate
				break
			}
		}
	}
	return RuntimeImageMetadata{ID: strings.TrimSpace(id), Digest: digest}, nil
}

// RuntimeComposeConfigInspect parses only project/service/image/volume names.
// Expanded service environment values from Compose are discarded in-process.
func (c *Client) RuntimeComposeConfigInspect(ctx context.Context, dir, project string) (RuntimeComposeConfig, error) {
	if !composeProjectPattern.MatchString(project) {
		return RuntimeComposeConfig{}, errors.New("invalid compose project")
	}
	_, raw, err := c.runWithEnvironmentRaw(ctx, "docker compose config", dir, c.timeouts.Ps, nil,
		"compose", "--project-name", project, "config", "--format", "json")
	if err != nil {
		return RuntimeComposeConfig{}, err
	}
	return parseRuntimeComposeConfig(raw, project)
}

// RuntimeComposeConfigInspectServer parses the existing Compose model while
// masking the disabled optional Auth image. Compose expands every declared
// service even when callers only operate on server; this keeps stale Auth
// configuration from becoming an indirect gate for disabled instances.
func (c *Client) RuntimeComposeConfigInspectServer(ctx context.Context, dir, project string) (RuntimeComposeConfig, error) {
	if !composeProjectPattern.MatchString(project) {
		return RuntimeComposeConfig{}, errors.New("invalid compose project")
	}
	_, raw, err := c.runWithEnvironmentRaw(ctx, "docker compose config server only", dir, c.timeouts.Ps,
		[]string{"STEAM_SERVICE_IMAGE=" + disabledSteamAuthImage},
		"compose", "--project-name", project, "config", "--format", "json")
	if err != nil {
		return RuntimeComposeConfig{}, err
	}
	return parseRuntimeComposeConfig(raw, project)
}

func parseRuntimeComposeConfig(output, project string) (RuntimeComposeConfig, error) {
	var payload struct {
		Name     string `json:"name"`
		Services map[string]struct {
			Image   string `json:"image"`
			Volumes []struct {
				Type   string `json:"type"`
				Source string `json:"source"`
				Target string `json:"target"`
			} `json:"volumes"`
		} `json:"services"`
		Volumes map[string]struct {
			Name string `json:"name"`
		} `json:"volumes"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		return RuntimeComposeConfig{}, errors.New("invalid docker compose config response")
	}
	config := RuntimeComposeConfig{Project: strings.TrimSpace(payload.Name), Images: map[string]string{}}
	for name, service := range payload.Services {
		config.Services = append(config.Services, name)
		config.Images[name] = strings.TrimSpace(service.Image)
		if name == "steam-auth" {
			for _, volume := range service.Volumes {
				if volume.Type != "volume" || volume.Target != "/data/steam-session" {
					continue
				}
				if declared, ok := payload.Volumes[volume.Source]; ok && dockerVolumePattern.MatchString(declared.Name) {
					config.SteamSessionVolume = declared.Name
				} else if dockerVolumePattern.MatchString(volume.Source) {
					config.SteamSessionVolume = project + "_" + volume.Source
				}
			}
		}
	}
	sort.Strings(config.Services)
	return config, nil
}

// RuntimeComposePsServer reads only the server container state while masking
// stale Auth image configuration. It is read-only and intentionally bypasses
// the general Compose status cache so an upgrade preflight sees current state.
func (c *Client) RuntimeComposePsServer(ctx context.Context, dir, project string) (ComposePsResult, error) {
	if !composeProjectPattern.MatchString(project) {
		return ComposePsResult{}, errors.New("invalid compose project")
	}
	result, raw, err := c.runWithEnvironmentRaw(ctx, "docker compose ps server only", dir, c.timeouts.Ps,
		[]string{"STEAM_SERVICE_IMAGE=" + disabledSteamAuthImage},
		"compose", "--project-name", project, "ps", "--format", "json", "server")
	composeResult := ComposePsResult{Result: result, Services: []ComposeService{}}
	if strings.TrimSpace(raw) != "" {
		services, parseErr := parseComposeServices(raw)
		if parseErr != nil {
			return composeResult, fmt.Errorf("parse docker compose ps server response: %w", parseErr)
		}
		composeResult.Services = services
	}
	return composeResult, err
}

// RuntimeComposeConfigValidateImages validates the existing Compose model with
// two controlled environment overrides. It never writes an env file and never
// executes compose up/down/restart/rm.
func (c *Client) RuntimeComposeConfigValidateImages(ctx context.Context, dir, project, serverImage, authImage string) error {
	if !composeProjectPattern.MatchString(project) {
		return errors.New("invalid compose project")
	}
	if err := validateRestrictedImageRef(serverImage); err != nil {
		return err
	}
	if err := validateRestrictedImageRef(authImage); err != nil {
		return err
	}
	_, err := c.runWithEnvironment(ctx, "docker compose config", dir, c.timeouts.Ps,
		[]string{"SERVER_IMAGE=" + serverImage, "STEAM_SERVICE_IMAGE=" + authImage},
		"compose", "--project-name", project, "config", "--quiet")
	return err
}

// RuntimeComposeConfigValidateServerImage validates the existing Compose model
// with only the always-required server image overridden. It does not validate,
// pull, create, or start the optional steam-auth service.
func (c *Client) RuntimeComposeConfigValidateServerImage(ctx context.Context, dir, project, serverImage string) error {
	if !composeProjectPattern.MatchString(project) {
		return errors.New("invalid compose project")
	}
	if err := validateRestrictedImageRef(serverImage); err != nil {
		return err
	}
	_, err := c.runWithEnvironment(ctx, "docker compose config", dir, c.timeouts.Ps,
		[]string{
			"SERVER_IMAGE=" + serverImage,
			// Compose parses every declared service even when the caller will only
			// operate on server. Mask a stale/invalid optional Auth image value so
			// disabled instances are not gated on that unused sidecar.
			"STEAM_SERVICE_IMAGE=" + disabledSteamAuthImage,
		},
		"compose", "--project-name", project, "config", "--quiet")
	return err
}

func (c *Client) RuntimeVolumeInspect(ctx context.Context, dir, volumeName string) (RuntimeVolumeMetadata, error) {
	if !dockerVolumePattern.MatchString(volumeName) {
		return RuntimeVolumeMetadata{}, errors.New("invalid docker volume name")
	}
	result, err := c.run(ctx, "docker volume inspect", dir, c.timeouts.Ps, "volume", "inspect", volumeName)
	if err != nil {
		return RuntimeVolumeMetadata{}, err
	}
	var payload []struct {
		Name       string `json:"Name"`
		Mountpoint string `json:"Mountpoint"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &payload); err != nil || len(payload) != 1 {
		return RuntimeVolumeMetadata{}, errors.New("invalid docker volume inspect response")
	}
	if payload[0].Name != volumeName {
		return RuntimeVolumeMetadata{}, errors.New("docker volume name mismatch")
	}
	return RuntimeVolumeMetadata{Name: payload[0].Name, Mountpoint: payload[0].Mountpoint}, nil
}

func validateRestrictedImageRef(imageRef string) error {
	if !restrictedImageRefPattern.MatchString(imageRef) || strings.Contains(imageRef, "@") || strings.HasSuffix(strings.ToLower(imageRef), ":latest") {
		return fmt.Errorf("invalid restricted image reference")
	}
	return nil
}

func (c *Client) runWithEnvironment(ctx context.Context, op, workDir string, timeout time.Duration, environment []string, args ...string) (CommandResult, error) {
	result, _, err := c.runWithEnvironmentRaw(ctx, op, workDir, timeout, environment, args...)
	return result, err
}

func (c *Client) runWithEnvironmentRedacted(ctx context.Context, op, workDir string, timeout time.Duration, environment []string, args ...string) (CommandResult, error) {
	result, raw, err := c.runWithEnvironmentRaw(ctx, op, workDir, timeout, environment, args...)
	result.Stdout = RedactString(raw)
	return result, err
}

// runWithEnvironmentRaw keeps stdout only in a private return value so callers
// can parse structured Docker output before generic redaction would corrupt
// JSON keys such as session. The raw value must never be logged or persisted;
// CommandResult deliberately contains no stdout.
func (c *Client) runWithEnvironmentRaw(ctx context.Context, op, workDir string, timeout time.Duration, environment []string, args ...string) (CommandResult, string, error) {
	started := time.Now()
	result := CommandResult{WorkDir: workDir, Args: RedactArgs(append([]string{c.dockerPath}, args...)), ExitCode: -1}
	if workDir == "" {
		return result, "", CommandError{Op: op, Result: result, Err: ErrInvalidWorkDir}
	}
	if stat, err := os.Stat(workDir); err != nil || !stat.IsDir() {
		return result, "", CommandError{Op: op, Result: result, Err: ErrInvalidWorkDir}
	}
	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(commandCtx, c.dockerPath, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), environment...)
	var stdout, stderr limitedBuffer
	stdout.limit, stderr.limit = c.maxOutputBytes, c.maxOutputBytes
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	result.DurationMS = time.Since(started).Milliseconds()
	rawStdout := stdout.String()
	// Compose output may contain expanded environment values. Never retain it.
	result.Stdout = ""
	result.Stderr = RedactString(stderr.String())
	result.StdoutTruncated = stdout.truncated
	result.StderrTruncated = stderr.truncated
	if commandCtx.Err() != nil {
		result.TimedOut = true
		return result, "", CommandError{Op: op, Result: result, Err: ErrCommandTimeout}
	}
	if err == nil {
		result.ExitCode = 0
		if result.StdoutTruncated {
			return result, "", CommandError{Op: op, Result: result, Err: errors.New("structured Docker output was truncated")}
		}
		return result, rawStdout, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, "", CommandError{Op: op, Result: result, Err: ErrCommandFailed}
	}
	return result, "", CommandError{Op: op, Result: result, Err: fmt.Errorf("start docker compose config: %w", err)}
}
