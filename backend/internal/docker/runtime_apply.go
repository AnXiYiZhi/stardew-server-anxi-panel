package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var runtimeContainerIDPattern = regexp.MustCompile(`^[a-f0-9]{12,64}$`)
var runtimeSnapshotVolumePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*_anxi-junimo-update-[a-f0-9]{24}-steam-session$`)
var runtimeHTTPStatusPattern = regexp.MustCompile(`^[0-9]{3}$`)

const runtimeAuthHealthProbe = `set -eu
exec 3<>/dev/tcp/127.0.0.1/3001
printf 'GET /health HTTP/1.0\r\nHost: 127.0.0.1\r\nConnection: close\r\n\r\n' >&3
IFS= read -r -t 1 status <&3 || exit 124
printf '%s\n' "$status"
while true; do
  IFS= read -r -t 1 line <&3 || exit 124
  [ "$line" = $'\r' ] && break
done
body=''
read_status=0
IFS= read -r -t 1 -d '' body <&3 || read_status=$?
[ "$read_status" -le 128 ] || exit 124
printf '%s' "$body"`

const runtimeServerHealthProbe = `set -eu
exec 3<>/dev/tcp/127.0.0.1/8080
printf 'GET /health HTTP/1.0\r\nHost: 127.0.0.1\r\nConnection: close\r\n\r\n' >&3
while IFS= read -r line <&3; do [ "$line" = $'\r' ] && break; done
cat <&3`

const runtimeVolumeCloneScript = `set -eu; cd /source; tar cf - . | tar xf - -C /target`
const runtimeVolumeRestoreScript = `set -eu; find /target -mindepth 1 -maxdepth 1 -exec rm -rf -- {} +; cd /source; tar cf - . | tar xf - -C /target`

type RuntimeServiceMetadata struct {
	ContainerID string `json:"containerId"`
	Image       string `json:"image"`
	ImageID     string `json:"imageId"`
	State       string `json:"state"`
	Health      string `json:"health,omitempty"`
}

type RuntimeAuthServiceHealth struct {
	LoggedIn     bool `json:"loggedIn"`
	AccountCount int  `json:"accountCount"`
}

type RuntimeAuthHealthError struct {
	Code    string
	Message string
}

func (e *RuntimeAuthHealthError) Error() string { return e.Code }

type runtimeAuthHealthEnvelope struct {
	Status   json.RawMessage `json:"status"`
	LoggedIn json.RawMessage `json:"logged_in"`
	Accounts json.RawMessage `json:"accounts"`
}

type RuntimeHostCapacity struct {
	CPUs        int
	MemoryBytes int64
}

func (c *Client) RuntimeHostCapacity(ctx context.Context, dir string) (RuntimeHostCapacity, error) {
	result, err := c.run(ctx, "inspect Docker host capacity", dir, c.timeouts.Ps,
		"info", "--format", `{{json .NCPU}}|{{json .MemTotal}}`)
	if err != nil {
		return RuntimeHostCapacity{}, err
	}
	return parseRuntimeHostCapacity(result.Stdout)
}

func parseRuntimeHostCapacity(output string) (RuntimeHostCapacity, error) {
	parts := strings.SplitN(strings.TrimSpace(output), "|", 2)
	if len(parts) != 2 {
		return RuntimeHostCapacity{}, errors.New("invalid Docker host capacity response")
	}
	var capacity RuntimeHostCapacity
	if json.Unmarshal([]byte(parts[0]), &capacity.CPUs) != nil || json.Unmarshal([]byte(parts[1]), &capacity.MemoryBytes) != nil || capacity.CPUs <= 0 || capacity.MemoryBytes <= 0 {
		return RuntimeHostCapacity{}, errors.New("invalid Docker host capacity response")
	}
	return capacity, nil
}

// RuntimeComposeStopServices stops only the fixed Junimo runtime services. It
// never removes containers or volumes and never accepts arbitrary services.
func (c *Client) RuntimeComposeStopServices(ctx context.Context, dir, project string, services ...string) error {
	if !composeProjectPattern.MatchString(project) || !validRuntimeServices(services) {
		return errors.New("invalid runtime compose stop request")
	}
	args := []string{"compose", "--project-name", project, "stop", "--timeout", "30"}
	args = append(args, services...)
	c.invalidateComposePs(dir)
	var err error
	if containsRuntimeServiceName(services, "steam-auth") {
		_, err = c.run(ctx, "docker compose stop runtime services", dir, c.timeouts.Down, args...)
	} else {
		_, err = c.runWithEnvironment(ctx, "docker compose stop server runtime", dir, c.timeouts.Down, []string{"STEAM_SERVICE_IMAGE=" + disabledSteamAuthImage}, args...)
	}
	c.invalidateComposePs(dir)
	return err
}

// RuntimeComposeUpService recreates exactly one Junimo runtime service without
// dependencies. The service name is a closed enum, not caller-controlled text.
func (c *Client) RuntimeComposeUpService(ctx context.Context, dir, project, service string) error {
	if !composeProjectPattern.MatchString(project) || !validRuntimeServices([]string{service}) {
		return errors.New("invalid runtime compose up request")
	}
	c.invalidateComposePs(dir)
	args := []string{"compose", "--project-name", project, "up", "-d", "--no-deps", "--force-recreate", "--pull", "never", service}
	var err error
	if service == "server" {
		_, err = c.runWithEnvironment(ctx, "docker compose up server runtime", dir, c.timeouts.Up, []string{"STEAM_SERVICE_IMAGE=" + disabledSteamAuthImage}, args...)
	} else {
		_, err = c.run(ctx, "docker compose up runtime service", dir, c.timeouts.Up, args...)
	}
	c.invalidateComposePs(dir)
	return err
}

// RuntimeComposeUpServicePreserve starts an unchanged runtime service without
// replacing an existing container. If no container exists, Compose may create
// one from the already inspected, pinned configuration.
func (c *Client) RuntimeComposeUpServicePreserve(ctx context.Context, dir, project, service string) error {
	if !composeProjectPattern.MatchString(project) || !validRuntimeServices([]string{service}) {
		return errors.New("invalid runtime compose preserve request")
	}
	c.invalidateComposePs(dir)
	args := []string{"compose", "--project-name", project, "up", "-d", "--no-deps", "--no-recreate", "--pull", "never", service}
	var err error
	if service == "server" {
		_, err = c.runWithEnvironment(ctx, "docker compose preserve server runtime", dir, c.timeouts.Up, []string{"STEAM_SERVICE_IMAGE=" + disabledSteamAuthImage}, args...)
	} else {
		_, err = c.run(ctx, "docker compose preserve runtime service", dir, c.timeouts.Up, args...)
	}
	c.invalidateComposePs(dir)
	return err
}

// RuntimeUpdateServiceCPUShares applies the standard Compose scheduling weight
// in place, preserving the container identity and all auth process/session
// state. Only the two reviewed runtime values are accepted.
func (c *Client) RuntimeUpdateServiceCPUShares(ctx context.Context, dir, project, service string, shares int64) error {
	want := int64(0)
	switch service {
	case "server":
		want = 768
	case "steam-auth":
		want = 256
	}
	if !composeProjectPattern.MatchString(project) || shares != want || want == 0 {
		return errors.New("invalid runtime cpu shares update")
	}
	args := []string{"compose", "--project-name", project, "ps", "-q", service}
	var ps CommandResult
	var err error
	if service == "server" {
		ps, err = c.runWithEnvironmentRedacted(ctx, "docker compose ps server runtime", dir, c.timeouts.Ps, []string{"STEAM_SERVICE_IMAGE=" + disabledSteamAuthImage}, args...)
	} else {
		ps, err = c.run(ctx, "docker compose ps runtime service", dir, c.timeouts.Ps, args...)
	}
	if err != nil {
		return err
	}
	containerID := strings.TrimSpace(ps.Stdout)
	if !runtimeContainerIDPattern.MatchString(containerID) {
		return errors.New("runtime service container not found")
	}
	_, err = c.run(ctx, "update runtime service cpu shares", dir, c.timeouts.Ps,
		"update", "--cpu-shares", fmt.Sprintf("%d", shares), containerID)
	return err
}

func (c *Client) RuntimeServiceInspect(ctx context.Context, dir, project, service string) (RuntimeServiceMetadata, error) {
	if !composeProjectPattern.MatchString(project) || !validRuntimeServices([]string{service}) {
		return RuntimeServiceMetadata{}, errors.New("invalid runtime service inspect request")
	}
	args := []string{"compose", "--project-name", project, "ps", "-q", service}
	var ps CommandResult
	var err error
	if service == "server" {
		ps, err = c.runWithEnvironmentRedacted(ctx, "docker compose ps server runtime", dir, c.timeouts.Ps, []string{"STEAM_SERVICE_IMAGE=" + disabledSteamAuthImage}, args...)
	} else {
		ps, err = c.run(ctx, "docker compose ps runtime service", dir, c.timeouts.Ps, args...)
	}
	if err != nil {
		return RuntimeServiceMetadata{}, err
	}
	containerID := strings.TrimSpace(ps.Stdout)
	if !runtimeContainerIDPattern.MatchString(containerID) {
		return RuntimeServiceMetadata{}, errors.New("runtime service container not found")
	}
	// Limit inspect output to fields that are safe to retain and parse. Full
	// container inspect JSON includes credentials in Config.Env and is also
	// vulnerable to structure-breaking redaction before parsing.
	result, err := c.run(ctx, "docker inspect runtime service", dir, c.timeouts.Ps,
		"inspect", "--format", `{{json .Image}}|{{json .Config.Image}}|{{json .State.Status}}|{{if .State.Health}}{{json .State.Health.Status}}{{else}}""{{end}}`, containerID)
	if err != nil {
		return RuntimeServiceMetadata{}, err
	}
	return parseRuntimeServiceInspectOutput(result.Stdout, containerID)
}

func parseRuntimeServiceInspectOutput(output, containerID string) (RuntimeServiceMetadata, error) {
	parts := strings.SplitN(strings.TrimSpace(output), "|", 4)
	if len(parts) != 4 {
		return RuntimeServiceMetadata{}, errors.New("invalid runtime service inspect response")
	}
	values := make([]string, 4)
	for index := range parts {
		if json.Unmarshal([]byte(parts[index]), &values[index]) != nil {
			return RuntimeServiceMetadata{}, errors.New("invalid runtime service inspect response")
		}
	}
	if !runtimeContainerIDPattern.MatchString(containerID) || !runtimeDigestPattern.MatchString(values[0]) || strings.TrimSpace(values[1]) == "" || strings.TrimSpace(values[2]) == "" {
		return RuntimeServiceMetadata{}, errors.New("invalid runtime service inspect response")
	}
	return RuntimeServiceMetadata{ContainerID: containerID, Image: strings.TrimSpace(values[1]), ImageID: values[0], State: strings.TrimSpace(values[2]), Health: strings.TrimSpace(values[3])}, nil
}

func (c *Client) RuntimeSteamAuthHealth(ctx context.Context, dir, project string) (RuntimeAuthServiceHealth, error) {
	if !composeProjectPattern.MatchString(project) {
		return RuntimeAuthServiceHealth{}, errors.New("invalid compose project")
	}
	result, err := c.run(ctx, "probe steam auth health", dir, c.timeouts.Ps,
		"compose", "--project-name", project, "exec", "-T", "steam-auth", "bash", "-c", runtimeAuthHealthProbe)
	if err != nil {
		return RuntimeAuthServiceHealth{}, runtimeAuthHealthCommandError(err)
	}
	return parseRuntimeAuthHealthHTTPResponse(result.Stdout)
}

func runtimeAuthHealthCommandError(err error) error {
	var commandErr CommandError
	if errors.Is(err, ErrCommandTimeout) || errors.As(err, &commandErr) && commandErr.Result.ExitCode == 124 {
		return newRuntimeAuthHealthError("auth_health_timeout", "steam-auth-cn /health 探针超时，未在单次探针预算内返回。")
	}
	return newRuntimeAuthHealthError("auth_health_unreachable", "steam-auth-cn /health 无法连接。")
}

func parseRuntimeAuthHealthHTTPResponse(output string) (RuntimeAuthServiceHealth, error) {
	statusLine, body, ok := strings.Cut(output, "\n")
	statusFields := strings.Fields(strings.TrimSpace(statusLine))
	if !ok || len(statusFields) < 2 || (statusFields[0] != "HTTP/1.0" && statusFields[0] != "HTTP/1.1") || !runtimeHTTPStatusPattern.MatchString(statusFields[1]) {
		return RuntimeAuthServiceHealth{}, newRuntimeAuthHealthError("auth_health_invalid_response", "steam-auth-cn /health 返回了无效的 HTTP/JSON 响应。")
	}
	if statusFields[1] != "200" {
		return RuntimeAuthServiceHealth{}, newRuntimeAuthHealthError("auth_health_http_status", fmt.Sprintf("steam-auth-cn /health 返回 HTTP %s；验收要求 HTTP 200。", statusFields[1]))
	}
	return parseRuntimeAuthHealthResponse(body)
}

func parseRuntimeAuthHealthResponse(output string) (RuntimeAuthServiceHealth, error) {
	var health runtimeAuthHealthEnvelope
	decoder := json.NewDecoder(strings.NewReader(output))
	if err := decoder.Decode(&health); err != nil {
		return RuntimeAuthServiceHealth{}, newRuntimeAuthHealthError("auth_health_invalid_response", "steam-auth-cn /health 返回了无效的 HTTP/JSON 响应。")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return RuntimeAuthServiceHealth{}, newRuntimeAuthHealthError("auth_health_invalid_response", "steam-auth-cn /health 返回了无效的 HTTP/JSON 响应。")
	}
	if health.Status == nil || health.LoggedIn == nil || health.Accounts == nil {
		return RuntimeAuthServiceHealth{}, newRuntimeAuthHealthError("auth_health_invalid_response", "steam-auth-cn /health 缺少 status、logged_in 或 accounts 字段。")
	}
	var status string
	if isRuntimeAuthNull(health.Status) || json.Unmarshal(health.Status, &status) != nil || status != "ok" {
		return RuntimeAuthServiceHealth{}, newRuntimeAuthHealthError("auth_health_invalid_response", "steam-auth-cn /health 的 status 必须是字符串 ok。")
	}
	loggedIn, err := decodeRuntimeAuthBool(health.LoggedIn)
	if err != nil {
		return RuntimeAuthServiceHealth{}, newRuntimeAuthHealthError("auth_health_invalid_response", "steam-auth-cn /health 的 logged_in 必须是布尔值。")
	}
	var accounts []json.RawMessage
	if !strings.HasPrefix(strings.TrimSpace(string(health.Accounts)), "[") || json.Unmarshal(health.Accounts, &accounts) != nil {
		return RuntimeAuthServiceHealth{}, newRuntimeAuthHealthError("auth_health_invalid_response", "steam-auth-cn /health 的 accounts 必须是 JSON 数组。")
	}
	return RuntimeAuthServiceHealth{LoggedIn: loggedIn, AccountCount: len(accounts)}, nil
}

func newRuntimeAuthHealthError(code, message string) error {
	return &RuntimeAuthHealthError{Code: code, Message: message}
}

func decodeRuntimeAuthBool(raw json.RawMessage) (bool, error) {
	if raw == nil || isRuntimeAuthNull(raw) {
		return false, errors.New("missing steam auth boolean")
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, err
	}
	return value, nil
}

func isRuntimeAuthNull(raw json.RawMessage) bool {
	return strings.EqualFold(strings.TrimSpace(string(raw)), "null")
}

func (c *Client) RuntimeServerHealth(ctx context.Context, dir, project string) error {
	if !composeProjectPattern.MatchString(project) {
		return errors.New("invalid compose project")
	}
	_, raw, err := c.runWithEnvironmentRaw(ctx, "probe Junimo health", dir, c.timeouts.Ps, []string{"STEAM_SERVICE_IMAGE=" + disabledSteamAuthImage},
		"compose", "--project-name", project, "exec", "-T", "server", "bash", "-c", runtimeServerHealthProbe)
	if err != nil {
		return err
	}
	var health struct {
		Status string `json:"status"`
	}
	if json.Unmarshal([]byte(raw), &health) != nil || !strings.EqualFold(health.Status, "ok") {
		return errors.New("Junimo health response is not ok")
	}
	return nil
}

func (c *Client) RuntimeCreateSnapshotVolume(ctx context.Context, dir, project, name string) error {
	if !composeProjectPattern.MatchString(project) || !runtimeSnapshotVolumePattern.MatchString(name) || !strings.HasPrefix(name, project+"_anxi-junimo-update-") {
		return errors.New("invalid runtime snapshot volume")
	}
	_, err := c.run(ctx, "create runtime snapshot volume", dir, c.timeouts.Version,
		"volume", "create", "--label", "com.anxi-panel.runtime-update-snapshot=true", "--label", "com.anxi-panel.compose-project="+project, name)
	return err
}

func (c *Client) RuntimeCloneVolume(ctx context.Context, dir, source, target, trustedImage string) error {
	if !dockerVolumePattern.MatchString(source) || !runtimeSnapshotVolumePattern.MatchString(target) || validateRestrictedImageRef(trustedImage) != nil {
		return errors.New("invalid runtime volume clone request")
	}
	_, err := c.run(ctx, "clone steam session volume", dir, c.timeouts.Up,
		"run", "--rm", "--network", "none", "--entrypoint", "sh",
		"--mount", "type=volume,src="+source+",dst=/source,readonly",
		"--mount", "type=volume,src="+target+",dst=/target",
		trustedImage, "-c", runtimeVolumeCloneScript)
	return err
}

func (c *Client) RuntimeRestoreVolume(ctx context.Context, dir, snapshot, target, trustedImage string) error {
	if !runtimeSnapshotVolumePattern.MatchString(snapshot) || !dockerVolumePattern.MatchString(target) || validateRestrictedImageRef(trustedImage) != nil {
		return errors.New("invalid runtime volume restore request")
	}
	_, err := c.run(ctx, "restore steam session volume", dir, c.timeouts.Up,
		"run", "--rm", "--network", "none", "--entrypoint", "sh",
		"--mount", "type=volume,src="+snapshot+",dst=/source,readonly",
		"--mount", "type=volume,src="+target+",dst=/target",
		trustedImage, "-c", runtimeVolumeRestoreScript)
	return err
}

func (c *Client) RuntimeRemoveSnapshotVolume(ctx context.Context, dir, project, name string) error {
	if !composeProjectPattern.MatchString(project) || !runtimeSnapshotVolumePattern.MatchString(name) || !strings.HasPrefix(name, project+"_anxi-junimo-update-") {
		return errors.New("invalid runtime snapshot volume removal")
	}
	_, err := c.run(ctx, "remove runtime snapshot volume", dir, c.timeouts.Version, "volume", "rm", name)
	return err
}

// RuntimeRemoveImage removes one exact, previously inspected runtime image
// reference after a successful transaction. It never forces removal and first
// verifies that the tag still resolves to the captured image ID, so a mutable
// tag race cannot delete newly published content.
func (c *Client) RuntimeRemoveImage(ctx context.Context, dir, imageRef, expectedImageID string) error {
	if validateRestrictedImageRef(imageRef) != nil || !runtimeDigestPattern.MatchString(expectedImageID) {
		return errors.New("invalid runtime image cleanup request")
	}
	metadata, err := c.RuntimeImageInspect(ctx, dir, imageRef)
	if err != nil {
		return err
	}
	if metadata.ID != expectedImageID {
		return errors.New("runtime image tag changed before cleanup")
	}
	references, err := c.run(ctx, "check runtime image container references", dir, c.timeouts.Version,
		"container", "ls", "--all", "--quiet", "--filter", "ancestor="+expectedImageID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(references.Stdout) != "" {
		return errors.New("runtime image is still referenced by a container")
	}
	_, err = c.run(ctx, "remove old runtime image", dir, c.timeouts.Version, "image", "rm", imageRef)
	return err
}

func validRuntimeServices(services []string) bool {
	if len(services) == 0 || len(services) > 2 {
		return false
	}
	seen := map[string]bool{}
	for _, service := range services {
		if service != "server" && service != "steam-auth" || seen[service] {
			return false
		}
		seen[service] = true
	}
	return true
}

func containsRuntimeServiceName(services []string, wanted string) bool {
	for _, service := range services {
		if service == wanted {
			return true
		}
	}
	return false
}
