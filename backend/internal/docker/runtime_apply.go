package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var runtimeContainerIDPattern = regexp.MustCompile(`^[a-f0-9]{12,64}$`)
var runtimeSnapshotVolumePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*_anxi-junimo-update-[a-f0-9]{24}-steam-session$`)

const runtimeAuthReadyProbe = `set -eu
exec 3<>/dev/tcp/127.0.0.1/3001
printf 'GET /steam/ready HTTP/1.0\r\nHost: 127.0.0.1\r\nConnection: close\r\n\r\n' >&3
IFS= read -r status <&3
printf '%s\n' "$status"
while IFS= read -r line <&3; do [ "$line" = $'\r' ] && break; done
cat <&3`

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

type RuntimeSteamReady struct {
	Ready     bool `json:"ready"`
	HasTicket bool `json:"hasTicket"`
}

type runtimeSteamReadyEnvelope struct {
	Ready     json.RawMessage `json:"ready"`
	HasTicket json.RawMessage `json:"has_ticket"`
	Status    json.RawMessage `json:"status"`
	LoggedIn  json.RawMessage `json:"logged_in"`
	Accounts  json.RawMessage `json:"accounts"`
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
	_, err := c.run(ctx, "docker compose stop runtime services", dir, c.timeouts.Down, args...)
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
	_, err := c.run(ctx, "docker compose up runtime service", dir, c.timeouts.Up,
		"compose", "--project-name", project, "up", "-d", "--no-deps", "--force-recreate", "--pull", "never", service)
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
	_, err := c.run(ctx, "docker compose preserve runtime service", dir, c.timeouts.Up,
		"compose", "--project-name", project, "up", "-d", "--no-deps", "--no-recreate", "--pull", "never", service)
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
	ps, err := c.run(ctx, "docker compose ps runtime service", dir, c.timeouts.Ps,
		"compose", "--project-name", project, "ps", "-q", service)
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
	ps, err := c.run(ctx, "docker compose ps runtime service", dir, c.timeouts.Ps,
		"compose", "--project-name", project, "ps", "-q", service)
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

func (c *Client) RuntimeSteamAuthReady(ctx context.Context, dir, project string) (RuntimeSteamReady, error) {
	if !composeProjectPattern.MatchString(project) {
		return RuntimeSteamReady{}, errors.New("invalid compose project")
	}
	result, err := c.run(ctx, "probe steam auth ready", dir, c.timeouts.Ps,
		"compose", "--project-name", project, "exec", "-T", "steam-auth", "bash", "-c", runtimeAuthReadyProbe)
	if err != nil {
		return RuntimeSteamReady{}, err
	}
	return parseRuntimeSteamReadyHTTPResponse(result.Stdout)
}

func parseRuntimeSteamReadyHTTPResponse(output string) (RuntimeSteamReady, error) {
	statusLine, body, ok := strings.Cut(output, "\n")
	statusFields := strings.Fields(strings.TrimSpace(statusLine))
	if !ok || len(statusFields) < 2 || (statusFields[0] != "HTTP/1.0" && statusFields[0] != "HTTP/1.1") {
		return RuntimeSteamReady{}, errors.New("invalid steam auth ready HTTP response")
	}
	if statusFields[1] == "503" {
		return parseRuntimeSteamUnavailableResponse(body)
	}
	if statusFields[1] != "200" {
		return RuntimeSteamReady{}, errors.New("steam auth ready endpoint returned an unsupported HTTP status")
	}
	return parseRuntimeSteamReadyResponse(body)
}

func parseRuntimeSteamUnavailableResponse(output string) (RuntimeSteamReady, error) {
	unavailable, err := decodeRuntimeSteamReadyEnvelope(output)
	if err != nil || unavailable.Ready == nil || unavailable.Status != nil || unavailable.LoggedIn != nil || unavailable.Accounts != nil {
		return RuntimeSteamReady{}, errors.New("invalid steam auth unavailable response")
	}
	ready, err := decodeRuntimeSteamBool(unavailable.Ready)
	if err != nil || ready {
		return RuntimeSteamReady{}, errors.New("invalid steam auth unavailable response")
	}
	if unavailable.HasTicket != nil {
		hasTicket, err := decodeRuntimeSteamBool(unavailable.HasTicket)
		if err != nil || hasTicket {
			return RuntimeSteamReady{}, errors.New("invalid steam auth unavailable response")
		}
	}
	return RuntimeSteamReady{Ready: false, HasTicket: false}, nil
}

func parseRuntimeSteamReadyResponse(output string) (RuntimeSteamReady, error) {
	ready, err := decodeRuntimeSteamReadyEnvelope(output)
	if err != nil {
		return RuntimeSteamReady{}, errors.New("invalid steam auth ready response")
	}
	// Keep accepting the original ready/has_ticket contract, but also accept
	// the current steam-service contract used by the reviewed auth image. Login
	// and ticket availability are capabilities for online play, not hard
	// runtime-upgrade acceptance requirements.
	if ready.Ready != nil {
		if ready.Status != nil || ready.LoggedIn != nil || ready.Accounts != nil {
			return RuntimeSteamReady{}, errors.New("mixed steam auth ready response")
		}
		legacyReady, err := decodeRuntimeSteamBool(ready.Ready)
		if err != nil {
			return RuntimeSteamReady{}, errors.New("invalid steam auth ready response")
		}
		hasTicket := false
		if ready.HasTicket != nil {
			hasTicket, err = decodeRuntimeSteamBool(ready.HasTicket)
			if err != nil {
				return RuntimeSteamReady{}, errors.New("invalid steam auth ready response")
			}
		}
		return RuntimeSteamReady{Ready: legacyReady, HasTicket: hasTicket}, nil
	}
	if ready.HasTicket != nil || ready.Status == nil || ready.LoggedIn == nil || ready.Accounts == nil {
		return RuntimeSteamReady{}, errors.New("incomplete steam auth ready response")
	}
	var status string
	if isRuntimeSteamNull(ready.Status) || json.Unmarshal(ready.Status, &status) != nil || !strings.EqualFold(strings.TrimSpace(status), "ok") {
		return RuntimeSteamReady{}, errors.New("invalid steam auth status response")
	}
	if _, err := decodeRuntimeSteamBool(ready.LoggedIn); err != nil {
		return RuntimeSteamReady{}, errors.New("invalid steam auth logged-in response")
	}
	var accounts []json.RawMessage
	if !strings.HasPrefix(strings.TrimSpace(string(ready.Accounts)), "[") || json.Unmarshal(ready.Accounts, &accounts) != nil {
		return RuntimeSteamReady{}, errors.New("invalid steam auth accounts response")
	}
	return RuntimeSteamReady{Ready: true, HasTicket: false}, nil
}

func decodeRuntimeSteamReadyEnvelope(output string) (runtimeSteamReadyEnvelope, error) {
	var envelope runtimeSteamReadyEnvelope
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		return runtimeSteamReadyEnvelope{}, err
	}
	return envelope, nil
}

func decodeRuntimeSteamBool(raw json.RawMessage) (bool, error) {
	if raw == nil || isRuntimeSteamNull(raw) {
		return false, errors.New("missing steam auth boolean")
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, err
	}
	return value, nil
}

func isRuntimeSteamNull(raw json.RawMessage) bool {
	return strings.EqualFold(strings.TrimSpace(string(raw)), "null")
}

func (c *Client) RuntimeServerHealth(ctx context.Context, dir, project string) error {
	if !composeProjectPattern.MatchString(project) {
		return errors.New("invalid compose project")
	}
	result, err := c.run(ctx, "probe Junimo health", dir, c.timeouts.Ps,
		"compose", "--project-name", project, "exec", "-T", "server", "bash", "-c", runtimeServerHealthProbe)
	if err != nil {
		return err
	}
	var health struct {
		Status string `json:"status"`
	}
	if json.Unmarshal([]byte(result.Stdout), &health) != nil || !strings.EqualFold(health.Status, "ok") {
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
