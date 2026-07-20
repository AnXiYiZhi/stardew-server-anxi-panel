package updater

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	containerReferencePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)
	composeProjectPattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,62}$`)
)

type MountInfo struct {
	Type        string `json:"Type"`
	Source      string `json:"Source"`
	Destination string `json:"Destination"`
}

type ContainerInfo struct {
	ID           string
	Name         string
	Image        string
	ImageID      string
	Labels       map[string]string
	Mounts       []MountInfo
	Env          []string
	User         string
	Privileged   bool
	NetworkMode  string
	PortBindings map[string][]struct {
		HostIP   string `json:"HostIp"`
		HostPort string `json:"HostPort"`
	}
}

type HelperSpec struct {
	Name            string
	RuntimeImage    string
	TargetVersion   string
	ComposeProject  string
	ComposeService  string
	HostInstallDir  string
	HostComposeFile string
	DataMount       string
	StateFile       string
}

type DockerRuntime interface {
	Available(context.Context) bool
	ComposeAvailable(context.Context) bool
	InspectContainer(context.Context, string) (ContainerInfo, error)
	StartHelper(context.Context, HelperSpec) error
	ImageDigest(context.Context, string) (string, error)
	StartApplyHelper(context.Context, ApplyHelperSpec) error
}

type ApplyHelperSpec struct {
	Name                 string
	RuntimeImage         string
	FromVersion          string
	TargetVersion        string
	OriginalDigest       string
	CurrentContainer     string
	ComposeProject       string
	ComposeService       string
	HostInstallDir       string
	HostComposeFile      string
	DataMount            string
	StateFile            string
	BackupDir            string
	DatabaseRelativePath string
	Conversion           bool
}

type DockerCLI struct {
	Path    string
	Timeout time.Duration
}

func NewDockerCLI() *DockerCLI { return &DockerCLI{Path: "docker", Timeout: 20 * time.Second} }

func (d *DockerCLI) command(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
	if timeout <= 0 {
		timeout = d.Timeout
	}
	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(commandCtx, d.Path, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &limitedDiscard{limit: 64 << 10}
	if err := cmd.Run(); err != nil {
		if commandCtx.Err() != nil {
			return nil, errors.New("docker command timed out")
		}
		return nil, errors.New("docker command failed")
	}
	return stdout.Bytes(), nil
}

func (d *DockerCLI) Available(ctx context.Context) bool {
	_, err := d.command(ctx, 10*time.Second, "version", "--format", "{{.Server.Version}}")
	return err == nil
}

func (d *DockerCLI) ComposeAvailable(ctx context.Context) bool {
	_, err := d.command(ctx, 10*time.Second, "compose", "version", "--short")
	return err == nil
}

func (d *DockerCLI) InspectContainer(ctx context.Context, ref string) (ContainerInfo, error) {
	if !containerReferencePattern.MatchString(ref) {
		return ContainerInfo{}, errors.New("invalid self container reference")
	}
	output, err := d.command(ctx, 15*time.Second, "inspect", ref)
	if err != nil {
		return ContainerInfo{}, err
	}
	var raw []struct {
		ID     string `json:"Id"`
		Name   string `json:"Name"`
		Image  string `json:"Image"`
		Config struct {
			Image  string            `json:"Image"`
			Labels map[string]string `json:"Labels"`
			Env    []string          `json:"Env"`
			User   string            `json:"User"`
		} `json:"Config"`
		HostConfig struct {
			Privileged   bool   `json:"Privileged"`
			NetworkMode  string `json:"NetworkMode"`
			PortBindings map[string][]struct {
				HostIP   string `json:"HostIp"`
				HostPort string `json:"HostPort"`
			} `json:"PortBindings"`
		} `json:"HostConfig"`
		Mounts []MountInfo `json:"Mounts"`
	}
	if err := json.Unmarshal(output, &raw); err != nil || len(raw) != 1 {
		return ContainerInfo{}, errors.New("invalid docker inspect response")
	}
	return ContainerInfo{
		ID: raw[0].ID, Name: strings.TrimPrefix(raw[0].Name, "/"), Image: raw[0].Config.Image, ImageID: raw[0].Image,
		Labels: raw[0].Config.Labels, Mounts: raw[0].Mounts,
		Env: raw[0].Config.Env, User: raw[0].Config.User, Privileged: raw[0].HostConfig.Privileged,
		NetworkMode: raw[0].HostConfig.NetworkMode, PortBindings: raw[0].HostConfig.PortBindings,
	}, nil
}

// ResolveComposeDeployment treats Docker Compose itself as the authority and
// proves that exactly one service resolves to this exact container, image and
// data mount. Labels are hints only and may be incomplete on FNOS.
func (d *DockerCLI) ResolveComposeDeployment(ctx context.Context, project, composeFile string, container ContainerInfo, dataDestination, dataSource string) (string, error) {
	if !composeProjectPattern.MatchString(project) || !filepath.IsAbs(composeFile) {
		return "", errors.New("invalid compose identity")
	}
	installDir := filepath.Dir(composeFile)
	runner := []string{"run", "--rm", "--network", "none", "--entrypoint", "docker",
		"--mount", "type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock",
		"--mount", fmt.Sprintf("type=bind,src=%s,dst=%s,readonly", installDir, installDir), container.Image}
	baseArgs := []string{"compose", "--project-name", project}
	if envFile := filepath.Join(filepath.Dir(composeFile), ".env"); fileExists(envFile) {
		baseArgs = append(baseArgs, "--env-file", envFile)
	}
	baseArgs = append(baseArgs, "-f", composeFile)
	raw, err := d.command(ctx, 2*time.Minute, append(append(append([]string{}, runner...), baseArgs...), "config", "--format", "json")...)
	if err != nil {
		return "", err
	}
	var config struct {
		Services map[string]struct {
			Image   string `json:"image"`
			Volumes []struct {
				Type   string `json:"type"`
				Source string `json:"source"`
				Target string `json:"target"`
			} `json:"volumes"`
		} `json:"services"`
	}
	if json.Unmarshal(raw, &config) != nil || len(config.Services) == 0 {
		return "", errors.New("invalid compose config")
	}
	matched := ""
	for service, definition := range config.Services {
		if !containerReferencePattern.MatchString(service) {
			continue
		}
		id, psErr := d.command(ctx, 30*time.Second, append(append(append([]string{}, runner...), baseArgs...), "ps", "-q", service)...)
		if psErr != nil || strings.TrimSpace(string(id)) != container.ID {
			continue
		}
		if matched != "" {
			return "", errors.New("container matches multiple compose services")
		}
		if strings.TrimSpace(definition.Image) != strings.TrimSpace(container.Image) {
			resolvedID, imageErr := d.command(ctx, 30*time.Second, "image", "inspect", "--format", "{{.Id}}", definition.Image)
			if imageErr != nil || strings.TrimSpace(string(resolvedID)) != strings.TrimSpace(container.ImageID) {
				return "", errors.New("compose image mismatch")
			}
		}
		mountMatches := 0
		for _, volume := range definition.Volumes {
			if filepath.Clean(volume.Target) == filepath.Clean(dataDestination) && filepath.Clean(volume.Source) == filepath.Clean(dataSource) {
				mountMatches++
			}
		}
		if mountMatches != 1 {
			return "", errors.New("compose data mount mismatch")
		}
		matched = service
	}
	if matched == "" {
		return "", errors.New("compose service not found")
	}
	return matched, nil
}

func (d *DockerCLI) ImageDigest(ctx context.Context, imageRef string) (string, error) {
	if strings.TrimSpace(imageRef) == "" || strings.HasPrefix(imageRef, "-") {
		return "", errors.New("invalid image reference")
	}
	output, err := d.command(ctx, 20*time.Second, "image", "inspect", "--format", "{{.Id}}", imageRef)
	if err != nil {
		return "", err
	}
	digest := strings.TrimSpace(string(output))
	if digest == "" || !strings.Contains(digest, "sha256:") {
		return "", errors.New("image digest unavailable")
	}
	return digest, nil
}

func BuildHelperArgs(spec HelperSpec) ([]string, error) {
	if strings.TrimSpace(spec.ComposeService) == "" {
		spec.ComposeService = "panel"
	}
	if !containerReferencePattern.MatchString(spec.Name) {
		return nil, errors.New("invalid helper name")
	}
	if strings.TrimSpace(spec.RuntimeImage) == "" || strings.HasPrefix(spec.RuntimeImage, "-") {
		return nil, errors.New("invalid helper runtime image")
	}
	version, err := NormalizeTargetVersion(spec.TargetVersion)
	if err != nil {
		return nil, err
	}
	if !composeProjectPattern.MatchString(spec.ComposeProject) || !containerReferencePattern.MatchString(spec.ComposeService) {
		return nil, errors.New("invalid compose project")
	}
	installDir := filepath.Clean(spec.HostInstallDir)
	composeFile := filepath.Clean(spec.HostComposeFile)
	if !filepath.IsAbs(installDir) || !filepath.IsAbs(composeFile) || filepath.Dir(composeFile) != installDir {
		return nil, errors.New("compose file must be directly inside install dir")
	}
	if strings.ContainsAny(installDir+composeFile, ",\r\n") {
		return nil, errors.New("deployment paths contain unsupported mount characters")
	}
	if strings.TrimSpace(spec.DataMount) == "" || strings.ContainsAny(spec.DataMount, ",\r\n") {
		return nil, errors.New("invalid data mount")
	}
	if spec.StateFile != "/data/updater/status.json" {
		return nil, errors.New("invalid updater state file")
	}
	args := []string{
		"run", "--detach", "--rm", "--name", spec.Name,
		"--label", "com.anxi-panel.updater=dry-run",
		"--entrypoint", "/app/panel-updater",
		"--mount", "type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock",
		// Keep the deployment directory at its host path inside the helper.
		// Docker Compose persists the client-side config path in container labels;
		// using a temporary /deployment path would make the next self-upgrade
		// point at a path that does not exist on the host.
		"--mount", fmt.Sprintf("type=bind,src=%s,dst=%s,readonly", installDir, installDir),
	}
	if filepath.IsAbs(spec.DataMount) {
		args = append(args, "--mount", fmt.Sprintf("type=bind,src=%s,dst=/data", filepath.Clean(spec.DataMount)))
	} else {
		if !containerReferencePattern.MatchString(spec.DataMount) {
			return nil, errors.New("invalid data volume")
		}
		args = append(args, "--mount", fmt.Sprintf("type=volume,src=%s,dst=/data", spec.DataMount))
	}
	args = append(args,
		spec.RuntimeImage,
		"dry-run",
		"--target-version", version,
		"--compose-project", spec.ComposeProject,
		"--compose-service", spec.ComposeService,
		"--compose-file", composeFile,
		"--state-file", spec.StateFile,
		"--current-image", spec.RuntimeImage,
	)
	return args, nil
}

func (d *DockerCLI) StartHelper(ctx context.Context, spec HelperSpec) error {
	args, err := BuildHelperArgs(spec)
	if err != nil {
		return err
	}
	_, err = d.command(ctx, 30*time.Second, args...)
	return err
}

func BuildApplyHelperArgs(spec ApplyHelperSpec) ([]string, error) {
	if strings.TrimSpace(spec.ComposeService) == "" {
		spec.ComposeService = "panel"
	}
	if !containerReferencePattern.MatchString(spec.Name) || !containerReferencePattern.MatchString(spec.CurrentContainer) {
		return nil, errors.New("invalid apply helper/container name")
	}
	if strings.TrimSpace(spec.RuntimeImage) == "" || strings.HasPrefix(spec.RuntimeImage, "-") {
		return nil, errors.New("invalid apply runtime image")
	}
	fromVersion, err := NormalizeTargetVersion(spec.FromVersion)
	if err != nil {
		return nil, err
	}
	targetVersion, err := NormalizeTargetVersion(spec.TargetVersion)
	if err != nil {
		return nil, err
	}
	if !composeProjectPattern.MatchString(spec.ComposeProject) || !containerReferencePattern.MatchString(spec.ComposeService) {
		return nil, errors.New("invalid compose project")
	}
	installDir := filepath.Clean(spec.HostInstallDir)
	composeFile := filepath.Clean(spec.HostComposeFile)
	if !filepath.IsAbs(installDir) || !filepath.IsAbs(composeFile) || filepath.Dir(composeFile) != installDir || strings.ContainsAny(installDir+composeFile, ",\r\n") {
		return nil, errors.New("invalid deployment paths")
	}
	if strings.TrimSpace(spec.DataMount) == "" || strings.ContainsAny(spec.DataMount, ",\r\n") {
		return nil, errors.New("invalid data mount")
	}
	if spec.StateFile != "/data/updater/apply-status.json" {
		return nil, errors.New("invalid apply state file")
	}
	if !strings.HasPrefix(filepath.ToSlash(filepath.Clean(spec.BackupDir)), "/data/updater/backups/") {
		return nil, errors.New("invalid apply backup dir")
	}
	dbRelative := filepath.Clean(spec.DatabaseRelativePath)
	if dbRelative == "." || filepath.IsAbs(dbRelative) || strings.HasPrefix(dbRelative, "..") || strings.ContainsAny(dbRelative, "\r\n") {
		return nil, errors.New("invalid database relative path")
	}
	args := []string{
		"run", "--detach", "--rm", "--name", spec.Name,
		"--label", "com.anxi-panel.updater=apply",
		"--entrypoint", "/app/panel-updater",
		"--mount", "type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock",
	}
	if spec.Conversion {
		if !filepath.IsAbs(spec.DataMount) {
			return nil, errors.New("legacy conversion requires a bind-mounted data directory")
		}
		hostParent := filepath.Dir(filepath.Clean(spec.DataMount))
		if hostParent == string(filepath.Separator) {
			return nil, errors.New("unsafe legacy conversion parent")
		}
		args = append(args, "--mount", fmt.Sprintf("type=bind,src=%s,dst=%s", hostParent, hostParent))
	} else {
		args = append(args, "--mount", fmt.Sprintf("type=bind,src=%s,dst=%s", installDir, installDir))
	}
	if filepath.IsAbs(spec.DataMount) {
		args = append(args, "--mount", fmt.Sprintf("type=bind,src=%s,dst=/data", filepath.Clean(spec.DataMount)))
	} else {
		if !containerReferencePattern.MatchString(spec.DataMount) {
			return nil, errors.New("invalid data volume")
		}
		args = append(args, "--mount", fmt.Sprintf("type=volume,src=%s,dst=/data", spec.DataMount))
	}
	operation := "apply"
	if spec.Conversion {
		operation = "convert"
	}
	args = append(args, spec.RuntimeImage, operation,
		"--from-version", fromVersion, "--target-version", targetVersion,
		"--current-image", spec.RuntimeImage, "--original-digest", spec.OriginalDigest,
		"--current-container", spec.CurrentContainer,
		"--compose-project", spec.ComposeProject,
		"--compose-service", spec.ComposeService,
		"--compose-file", composeFile,
		"--state-file", spec.StateFile, "--backup-dir", spec.BackupDir,
		"--database-relative", filepath.ToSlash(dbRelative),
	)
	return args, nil
}

func (d *DockerCLI) StartApplyHelper(ctx context.Context, spec ApplyHelperSpec) error {
	args, err := BuildApplyHelperArgs(spec)
	if err != nil {
		return err
	}
	_, err = d.command(ctx, 30*time.Second, args...)
	return err
}

type limitedDiscard struct{ limit int }

func (w *limitedDiscard) Write(p []byte) (int, error) {
	if w.limit > 0 {
		if len(p) > w.limit {
			w.limit = 0
		} else {
			w.limit -= len(p)
		}
	}
	return len(p), nil
}
