package updater

import (
	"context"
	"path/filepath"
	"strings"
)

const (
	labelProject     = "com.docker.compose.project"
	labelWorkingDir  = "com.docker.compose.project.working_dir"
	labelConfigFiles = "com.docker.compose.project.config_files"
	labelService     = "com.docker.compose.service"
)

type DetectOptions struct {
	ContainerRef     string
	ContainerDataDir string
	HostInstallDir   string
	HostComposeFile  string
	HostDataDir      string
	ComposeProject   string
}

func DetectCapability(ctx context.Context, docker DockerRuntime, opts DetectOptions) Capability {
	if !docker.Available(ctx) {
		return Unsupported(CodeDockerUnavailable, "Docker Socket 不可用或无法连接 Docker daemon")
	}
	base := Capability{DockerAvailable: true}
	if !docker.ComposeAvailable(ctx) {
		base.Code, base.Reason = CodeComposeUnavailable, "Docker Compose plugin 不可用"
		return base
	}
	base.ComposeAvailable = true
	info, err := docker.InspectContainer(ctx, strings.TrimSpace(opts.ContainerRef))
	if err != nil {
		base.Code, base.Reason = CodeSelfInspectFailed, "无法通过 HOSTNAME/container ID 安全识别当前面板容器"
		return base
	}
	base.CurrentContainer = info.Name
	if base.CurrentContainer == "" && len(info.ID) >= 12 {
		base.CurrentContainer = info.ID[:12]
	}
	base.CurrentImage = info.Image
	if !hasMountDestination(info.Mounts, "/var/run/docker.sock") {
		base.Code, base.Reason = CodeDockerSocketMissing, "当前容器没有可验证的 Docker Socket 挂载"
		return base
	}
	dataMount := mountSourceAt(info.Mounts, opts.ContainerDataDir)
	if dataMount == "" {
		base.Code, base.Reason = CodeDataMountMissing, "无法识别面板数据目录对应的宿主机挂载"
		return base
	}
	base.DataMount = dataMount

	labels := info.Labels
	project := strings.TrimSpace(labels[labelProject])
	service := strings.TrimSpace(labels[labelService])
	configFiles := splitComposeFiles(labels[labelConfigFiles])
	if project != "" || service != "" || len(configFiles) > 0 {
		if !composeProjectPattern.MatchString(project) || service != "panel" || len(configFiles) != 1 {
			base.Code, base.Reason = CodeComposeMetadataInvalid, "当前容器的 Compose labels 不完整或不符合标准 panel 服务"
			return base
		}
		composeFile := filepath.Clean(configFiles[0])
		if !filepath.IsAbs(composeFile) {
			workingDir := filepath.Clean(strings.TrimSpace(labels[labelWorkingDir]))
			if !filepath.IsAbs(workingDir) {
				base.Code, base.Reason = CodeComposeMetadataInvalid, "Compose config_files 不是可验证的绝对路径"
				return base
			}
			composeFile = filepath.Join(workingDir, composeFile)
		}
		base.ComposeProject = project
		base.ComposeFile = composeFile
		base.InstallDir = filepath.Dir(composeFile)
	} else {
		if !explicitDeploymentValid(opts, dataMount) {
			base.Code, base.Reason = CodeComposeLabelsMissing, "缺少标准 Compose labels，且未提供完整可靠的宿主机部署变量"
			return base
		}
		base.ComposeProject = strings.TrimSpace(opts.ComposeProject)
		base.ComposeFile = filepath.Clean(opts.HostComposeFile)
		base.InstallDir = filepath.Clean(opts.HostInstallDir)
	}
	if strings.TrimSpace(base.CurrentImage) == "" {
		base.Code, base.Reason = CodeComposeMetadataInvalid, "当前容器镜像引用为空"
		return base
	}
	base.Supported = true
	base.Code = CodeSupported
	base.Reason = "已识别标准 Compose 部署，可安全执行只读升级演练"
	return base
}

func explicitDeploymentValid(opts DetectOptions, actualDataMount string) bool {
	installDir := filepath.Clean(strings.TrimSpace(opts.HostInstallDir))
	composeFile := filepath.Clean(strings.TrimSpace(opts.HostComposeFile))
	hostData := filepath.Clean(strings.TrimSpace(opts.HostDataDir))
	return filepath.IsAbs(installDir) && filepath.IsAbs(composeFile) && filepath.Dir(composeFile) == installDir &&
		composeProjectPattern.MatchString(strings.TrimSpace(opts.ComposeProject)) && hostData == filepath.Clean(actualDataMount)
}

func splitComposeFiles(value string) []string {
	var result []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}

func hasMountDestination(mounts []MountInfo, destination string) bool {
	return mountSourceAt(mounts, destination) != ""
}

func mountSourceAt(mounts []MountInfo, destination string) string {
	for _, mount := range mounts {
		if filepath.Clean(mount.Destination) == filepath.Clean(destination) {
			return strings.TrimSpace(mount.Source)
		}
	}
	return ""
}
