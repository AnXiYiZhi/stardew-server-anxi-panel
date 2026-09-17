package stardew_junimo

import (
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"path/filepath"
	"strings"
)

func (d *Driver) ResourceStorage(instance registry.Instance) (registry.ResourceStorage, error) {
	volume, err := GameDataVolumeName(instance.DataDir)
	if err != nil {
		return registry.ResourceStorage{}, err
	}
	project := strings.ToLower(filepath.Base(filepath.Clean(instance.DataDir)))
	login, home := d.sharedSteamAuthorizationVolumes(instance)
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		return registry.ResourceStorage{}, err
	}
	image := InspectRuntimeStack(instance.DataDir, instance.State).Current.Server.Image
	if image == "" {
		image = manifest.Server.Image
	}
	resources := registry.ResourceStorage{
		Directories:   []string{instance.DataDir},
		Volumes:       []string{volume, project + "_steam-session"},
		SharedVolumes: []string{login, home},
		ProbeImage:    image,
	}
	if d.steamDownloads != nil {
		resources.SharedDirectories = []string{filepath.Dir(d.steamDownloads.CredentialsPath())}
	}
	return resources, nil
}
