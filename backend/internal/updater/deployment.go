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

type composeDeploymentResolver interface {
	ResolveComposeDeployment(context.Context, string, string, ContainerInfo, string, string) (string, error)
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
		if project == "" {
			project = strings.TrimSpace(opts.ComposeProject)
		}
		if len(configFiles) == 0 && strings.TrimSpace(opts.HostComposeFile) != "" {
			configFiles = []string{strings.TrimSpace(opts.HostComposeFile)}
		}
		if !composeProjectPattern.MatchString(project) || len(configFiles) != 1 {
			base.Code, base.Reason = CodeComposeMetadataInvalid, "当前容器的 Compose 元数据不足，无法反查唯一部署"
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
			return legacyConversionCapability(base, info, opts.ContainerDataDir, dataMount)
		}
		base.ComposeProject = strings.TrimSpace(opts.ComposeProject)
		base.ComposeFile = filepath.Clean(opts.HostComposeFile)
		base.InstallDir = filepath.Clean(opts.HostInstallDir)
	}
	resolver, ok := docker.(composeDeploymentResolver)
	if !ok {
		base.Code, base.Reason = CodeComposeMetadataInvalid, "Docker 驱动不支持 Compose 反向一致性校验"
		return base
	}
	resolvedService, err := resolver.ResolveComposeDeployment(ctx, base.ComposeProject, base.ComposeFile, info, opts.ContainerDataDir, dataMount)
	if err != nil || service != "" && service != resolvedService {
		base.Code, base.Reason = CodeComposeMetadataInvalid, "容器、Compose 文件、服务、镜像和数据挂载无法全部反向匹配"
		return base
	}
	base.ComposeService = resolvedService
	if strings.TrimSpace(base.CurrentImage) == "" {
		base.Code, base.Reason = CodeComposeMetadataInvalid, "当前容器镜像引用为空"
		return base
	}
	base.Supported = true
	base.Code = CodeSupported
	base.Reason = "已反向确认容器、Compose 文件、服务、镜像和数据挂载完全一致，可安全升级"
	return base
}

func legacyConversionCapability(base Capability, info ContainerInfo, dataDestination, dataMount string) Capability {
	data := mountAt(info.Mounts, dataDestination)
	socket := mountAt(info.Mounts, "/var/run/docker.sock")
	hasSecret := false
	for _, entry := range info.Env {
		if strings.HasPrefix(entry, "PANEL_SECRET=") && strings.TrimSpace(strings.TrimPrefix(entry, "PANEL_SECRET=")) != "" {
			hasSecret = true
		}
	}
	ports := info.PortBindings["8090/tcp"]
	if data.Type != "bind" || !filepath.IsAbs(dataMount) || filepath.Clean(dataMount) == string(filepath.Separator) ||
		socket.Type != "bind" || filepath.Clean(socket.Source) != filepath.Clean("/var/run/docker.sock") || len(info.Mounts) != 2 ||
		info.Privileged || strings.TrimSpace(info.User) != "" || !hasSecret || len(ports) == 0 || strings.TrimSpace(info.Image) == "" {
		base.Code, base.Reason = CodeComposeMetadataInvalid, "非标准部署未通过安全转换前置检查；容器、端口、权限、环境或挂载存在无法保真的配置"
		return base
	}
	installDir := filepath.Dir(filepath.Clean(dataMount))
	if filepath.Base(filepath.Clean(dataMount)) != "data" {
		installDir = filepath.Join(installDir, ".anxi-panel-"+info.Name)
	}
	base.Supported, base.Code, base.ConversionRequired = true, CodeSupported, true
	base.ComposeProject, base.ComposeService = legacyComposeProject(info.Name), "panel"
	base.InstallDir, base.ComposeFile = installDir, filepath.Join(installDir, "docker-compose.yml")
	base.Reason = "检测到可安全转换的飞牛非标准部署；升级时将由独立 helper 生成标准 Compose，并保留旧容器用于自动回滚"
	return base
}

func legacyComposeProject(containerName string) string {
	var suffix strings.Builder
	for _, r := range strings.ToLower(containerName) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			suffix.WriteRune(r)
		}
		if suffix.Len() >= 48 {
			break
		}
	}
	if suffix.Len() == 0 {
		return "anxi-panel-managed"
	}
	return "anxi-panel-" + suffix.String()
}

func mountAt(mounts []MountInfo, destination string) MountInfo {
	for _, mount := range mounts {
		if filepath.Clean(mount.Destination) == filepath.Clean(destination) {
			return mount
		}
	}
	return MountInfo{}
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
