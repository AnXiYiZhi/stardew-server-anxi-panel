package stardew_junimo

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	serverContEnvDir         = ".local-container/cont-env"
	serverContGroupsDir      = ".local-container/cont-groups"
	serverContUsersDir       = ".local-container/cont-users"
	serverAppNameContEnvFile = serverContEnvDir + "/APP_NAME"
)

type serverStaticInitValue struct {
	localPath     string
	containerPath string
	value         string
}

var serverStaticInitValues = []serverStaticInitValue{
	{serverContEnvDir + "/APP_NAME", "/etc/cont-env.d/APP_NAME", "DockerApp"},
	{serverContEnvDir + "/DBUS_SESSION_BUS_ADDRESS", "/etc/cont-env.d/DBUS_SESSION_BUS_ADDRESS", "unix:path=/tmp/dbus.base"},
	{serverContEnvDir + "/DOCKER_IMAGE_PLATFORM", "/etc/cont-env.d/DOCKER_IMAGE_PLATFORM", "linux/amd64"},
	{serverContEnvDir + "/GTK_A11Y", "/etc/cont-env.d/GTK_A11Y", "none"},
	{serverContEnvDir + "/NO_AT_BRIDGE", "/etc/cont-env.d/NO_AT_BRIDGE", "1"},
	{serverContEnvDir + "/TAKE_CONFIG_OWNERSHIP", "/etc/cont-env.d/TAKE_CONFIG_OWNERSHIP", "1"},
	{serverContEnvDir + "/XDG_CACHE_HOME", "/etc/cont-env.d/XDG_CACHE_HOME", "/config/xdg/cache"},
	{serverContEnvDir + "/XDG_CONFIG_HOME", "/etc/cont-env.d/XDG_CONFIG_HOME", "/config/xdg/config"},
	{serverContEnvDir + "/XDG_DATA_HOME", "/etc/cont-env.d/XDG_DATA_HOME", "/config/xdg/data"},
	{serverContEnvDir + "/XDG_RUNTIME_DIR", "/etc/cont-env.d/XDG_RUNTIME_DIR", "/tmp/run/user/app"},
	{serverContEnvDir + "/XDG_STATE_HOME", "/etc/cont-env.d/XDG_STATE_HOME", "/config/xdg/state"},
	{serverContGroupsDir + "/cinit/id", "/etc/cont-groups.d/cinit/id", "72"},
	{serverContGroupsDir + "/nogroup/id", "/etc/cont-groups.d/nogroup/id", "65534"},
	{serverContGroupsDir + "/root/id", "/etc/cont-groups.d/root/id", "0"},
	{serverContGroupsDir + "/shadow/id", "/etc/cont-groups.d/shadow/id", "42"},
	{serverContGroupsDir + "/staff/id", "/etc/cont-groups.d/staff/id", "52"},
	{serverContUsersDir + "/_apt/gid", "/etc/cont-users.d/_apt/gid", "65534"},
	{serverContUsersDir + "/_apt/home", "/etc/cont-users.d/_apt/home", "/nonexistent"},
	{serverContUsersDir + "/_apt/id", "/etc/cont-users.d/_apt/id", "105"},
	{serverContUsersDir + "/root/gid", "/etc/cont-users.d/root/gid", "0"},
	{serverContUsersDir + "/root/grps", "/etc/cont-users.d/root/grps", "root"},
	{serverContUsersDir + "/root/home", "/etc/cont-users.d/root/home", "/root"},
	{serverContUsersDir + "/root/id", "/etc/cont-users.d/root/id", "0"},
}

// EnsureServerContEnvFix masks malformed static init values in the
// JunimoServer image. Some upstream builds contain bare values such as
// "DockerApp", "linux/amd64", or numeric IDs under /etc/cont-*.d; the init
// loader executes those files as shell commands before loading their output.
func EnsureServerContEnvFix(dataDir string) (bool, error) {
	if dataDir == "" {
		return false, nil
	}
	changed := false
	for _, staticValue := range serverStaticInitValues {
		target := filepath.Join(dataDir, filepath.FromSlash(staticValue.localPath))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return false, err
		}
		script := serverStaticInitScript(staticValue.value)
		if current, err := os.ReadFile(target); err != nil || string(current) != script {
			if err := os.WriteFile(target, []byte(script), 0o755); err != nil {
				return false, err
			}
			changed = true
		} else if runtime.GOOS != "windows" {
			info, err := os.Stat(target)
			if err != nil || info.Mode().Perm() == 0o755 {
				continue
			}
			if err := os.Chmod(target, 0o755); err != nil {
				return false, err
			}
			changed = true
		}
	}

	composeChanged, err := migrateServerContEnvFixMount(filepath.Join(dataDir, "docker-compose.yml"))
	if err != nil {
		return changed, err
	}
	return changed || composeChanged, nil
}

func serverStaticInitScript(value string) string {
	return "#!/bin/sh\nprintf '%s\\n' " + shellSingleQuote(value) + "\n"
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func serverStaticInitComposeMount(staticValue serverStaticInitValue) string {
	return "      - ./" + staticValue.localPath + ":" + staticValue.containerPath + ":ro"
}

func migrateServerContEnvFixMount(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	text := string(raw)

	var missing []string
	for _, staticValue := range serverStaticInitValues {
		if strings.Contains(text, staticValue.containerPath) {
			continue
		}
		missing = append(missing, serverStaticInitComposeMount(staticValue))
	}
	if len(missing) == 0 {
		return false, nil
	}
	mountBlock := strings.Join(missing, "\n")

	for _, marker := range []string{
		"      - ./.local-container/cont-env/APP_NAME:/etc/cont-env.d/APP_NAME:ro",
		"      - ./.local-container/control:/data/control",
		"      - ./.local-container/mods/StardewAnxiPanel.Control:/data/Mods/StardewAnxiPanel.Control",
		"      - ./.local-container/settings:/data/settings",
	} {
		updated := insertLineAfter(text, marker, mountBlock)
		if updated == text {
			continue
		}
		info, statErr := os.Stat(path)
		mode := os.FileMode(0o644)
		if statErr == nil {
			mode = info.Mode().Perm()
		}
		if err := os.WriteFile(path, []byte(updated), mode); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}
