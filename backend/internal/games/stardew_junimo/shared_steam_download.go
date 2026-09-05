package stardew_junimo

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	sharedsteamcmd "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/steamcmd"
)

func (d *Driver) sharedSteamCredentials(instance registry.Instance) (sharedsteamcmd.Credentials, bool, error) {
	if d.steamDownloads == nil {
		values, err := sjconfig.ReadEnvFile(filepath.Join(instance.DataDir, ".env"))
		if err != nil {
			return sharedsteamcmd.Credentials{}, false, err
		}
		credentials := sharedsteamcmd.Credentials{
			Username:               strings.TrimSpace(values["STEAM_USERNAME"]),
			Password:               values["STEAM_PASSWORD"],
			AuthorizationCompleted: strings.EqualFold(strings.TrimSpace(values["STEAMCMD_AUTH_COMPLETED"]), "true"),
		}
		return credentials, credentials.Username != "" && credentials.Password != "", nil
	}
	credentials, found, err := d.steamDownloads.Load()
	if err != nil || found {
		return credentials, found, err
	}
	values, err := sjconfig.ReadEnvFile(filepath.Join(instance.DataDir, ".env"))
	if err != nil {
		return sharedsteamcmd.Credentials{}, false, err
	}
	legacy := sharedsteamcmd.Credentials{
		Username:               strings.TrimSpace(values["STEAM_USERNAME"]),
		Password:               values["STEAM_PASSWORD"],
		AuthorizationCompleted: strings.EqualFold(strings.TrimSpace(values["STEAMCMD_AUTH_COMPLETED"]), "true"),
	}
	if migrated, err := d.steamDownloads.SaveIfMissing(legacy); err != nil {
		return sharedsteamcmd.Credentials{}, false, fmt.Errorf("migrate shared Steam credentials: %w", err)
	} else if migrated {
		return legacy, true, nil
	}
	return sharedsteamcmd.Credentials{}, false, nil
}

func (d *Driver) saveSharedSteamCredentials(instance registry.Instance, username, password string) error {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return fmt.Errorf("Steam 用户名和密码不能为空")
	}
	if d.steamDownloads == nil {
		return sjconfig.UpdateEnvFile(filepath.Join(instance.DataDir, ".env"), map[string]string{
			"STEAM_USERNAME": username,
			"STEAM_PASSWORD": password,
		})
	}
	current, found, err := d.sharedSteamCredentials(instance)
	if err != nil {
		return err
	}
	completed := found && current.Username == username && current.AuthorizationCompleted
	return d.steamDownloads.Save(sharedsteamcmd.Credentials{
		Username: username, Password: password, AuthorizationCompleted: completed,
	})
}

func (d *Driver) setSharedSteamAuthorizationCompleted(instance registry.Instance, completed bool) error {
	if d.steamDownloads == nil {
		value := ""
		if completed {
			value = "true"
		}
		return sjconfig.UpdateEnvFile(filepath.Join(instance.DataDir, ".env"), map[string]string{"STEAMCMD_AUTH_COMPLETED": value})
	}
	return d.steamDownloads.SetAuthorizationCompleted(completed)
}

func (d *Driver) sharedSteamAuthorizationVolumes(instance registry.Instance) (login, home string) {
	if d.steamDownloads != nil {
		return d.steamDownloads.AuthorizationVolumeNames()
	}
	project := strings.ToLower(filepath.Base(instance.DataDir))
	return project + "_steamcmd-login", project + "_steamcmd-home"
}

func (d *Driver) acquireSharedSteamDownload(ctx context.Context) (func(), error) {
	if d.steamDownloads == nil {
		return func() {}, nil
	}
	return d.steamDownloads.AcquireDownload(ctx)
}

// StoredSteamDownloadCredentials is intentionally an internal driver
// capability. The web layer uses it to start download/invite jobs without
// reading instance .env files; no response exposes the password.
func (d *Driver) StoredSteamDownloadCredentials(instance registry.Instance) (username, password string, found bool, err error) {
	credentials, found, err := d.sharedSteamCredentials(instance)
	return credentials.Username, credentials.Password, found, err
}

// SteamDownloadStatus exposes only non-secret Panel-wide download state.
func (d *Driver) SteamDownloadStatus(instance registry.Instance) (configured, authorizationCached bool, err error) {
	credentials, found, err := d.sharedSteamCredentials(instance)
	if err != nil {
		return false, false, err
	}
	return found, found && credentials.AuthorizationCompleted, nil
}
