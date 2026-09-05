package steamcmd

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

var (
	volumeNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)
	installDirPattern = regexp.MustCompile(`^/data/game(?:/[A-Za-z0-9_.-]+)*$`)
)

type AppDownload struct {
	AppID      int
	InstallDir string
	Anonymous  bool
}

type ContainerRequest struct {
	ImageRef      string
	TargetVolume  string
	LoginVolume   string
	HomeVolume    string
	Username      string
	Password      string
	UseCachedAuth bool
	Applications  []AppDownload
}

// BuildContainerOptions produces the shared, short-lived SteamCMD executor
// contract. Game drivers supply only their app manifest; credentials, cache
// mounts and HOME normalization stay common.
func BuildContainerOptions(request ContainerRequest) (paneldocker.ContainerTTYRunOpts, error) {
	if strings.TrimSpace(request.ImageRef) == "" || !volumeNamePattern.MatchString(request.TargetVolume) ||
		!volumeNamePattern.MatchString(request.LoginVolume) || !volumeNamePattern.MatchString(request.HomeVolume) ||
		strings.TrimSpace(request.Username) == "" || request.Password == "" || len(request.Applications) == 0 {
		return paneldocker.ContainerTTYRunOpts{}, errors.New("invalid shared SteamCMD container request")
	}
	commands := make([]string, 0, len(request.Applications)*2)
	for _, app := range request.Applications {
		if app.AppID <= 0 || !installDirPattern.MatchString(app.InstallDir) {
			return paneldocker.ContainerTTYRunOpts{}, errors.New("invalid SteamCMD application manifest")
		}
		login := `+login "$STEAM_USERNAME" "$STEAM_PASSWORD"`
		prefix := ""
		if app.Anonymous {
			login = "+login anonymous"
		} else if request.UseCachedAuth {
			prefix = "+@NoPromptForPassword 1 "
			login = `+login "$STEAM_USERNAME"`
		}
		command := fmt.Sprintf(`"$STEAMCMD_BIN" %s+force_install_dir %s %s +app_update %s validate +quit`, prefix, app.InstallDir, login, strconv.Itoa(app.AppID))
		withHome := `HOME=/home/steam USER=steam LOGNAME=steam ` + command
		commands = append(commands,
			fmt.Sprintf(`echo "Running SteamCMD app_update %d..."`, app.AppID),
			`if id steam >/dev/null 2>&1 && command -v su >/dev/null 2>&1; then su -m steam -c '`+strings.ReplaceAll(withHome, `'`, `'"'"'`)+`'; else `+command+`; fi`,
		)
	}
	script := strings.Join(append([]string{
		"set -e",
		"mkdir -p /data/game /home/steam/Steam /home/steam/.steam /home/steam/.local/share/Steam /root/Steam /root/.steam /root/.local/share/Steam",
		"if id steam >/dev/null 2>&1; then chown -R steam:steam /data/game /home/steam/Steam /home/steam/.steam /home/steam/.local/share/Steam /root/Steam /root/.steam /root/.local/share/Steam; fi",
		`if [ -x /home/steam/steamcmd/steamcmd.sh ]; then steamcmd_bin=/home/steam/steamcmd/steamcmd.sh; elif command -v steamcmd >/dev/null 2>&1; then steamcmd_bin=$(command -v steamcmd); elif [ -x /usr/games/steamcmd ]; then steamcmd_bin=/usr/games/steamcmd; elif [ -x /steamcmd/steamcmd.sh ]; then steamcmd_bin=/steamcmd/steamcmd.sh; else echo "SteamCMD executable not found in container" >&2; exit 127; fi`,
		`export STEAMCMD_BIN="$steamcmd_bin"`,
	}, commands...), "; ")
	return paneldocker.ContainerTTYRunOpts{
		ImageRef: request.ImageRef, Entrypoint: []string{"/bin/sh"}, User: "root", Command: []string{"-lc", script},
		Env: []string{"STEAM_USERNAME=" + request.Username, "STEAM_PASSWORD=" + request.Password},
		Binds: []string{
			request.LoginVolume + ":/home/steam/Steam",
			request.LoginVolume + ":/home/steam/.local/share/Steam",
			request.HomeVolume + ":/home/steam/.steam",
			request.LoginVolume + ":/root/Steam",
			request.LoginVolume + ":/root/.local/share/Steam",
			request.HomeVolume + ":/root/.steam",
			request.TargetVolume + ":/data/game",
		},
	}, nil
}
