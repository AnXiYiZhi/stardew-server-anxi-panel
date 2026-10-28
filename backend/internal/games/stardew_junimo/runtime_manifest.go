package stardew_junimo

import (
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

// Production drivers have no provider override. Keeping the ordinary runtime
// reader scoped to a driver lets historical protocol fixtures retain their
// exact matrix without mutating the process-wide embedded recommendation.
// Installation and coordinated full-stack authorization read the embedded
// manifest directly and do not use this dependency.
func (d *Driver) runtimeStackManifest() (sjconfig.RuntimeStackManifest, error) {
	if d.runtimeManifestProvider != nil {
		return d.runtimeManifestProvider()
	}
	return sjconfig.BuiltInRuntimeStackManifest()
}

func (d *Driver) inspectRuntimeStack(instance registry.Instance) sjconfig.RuntimeStackInspection {
	manifest, err := d.runtimeStackManifest()
	if err != nil {
		return sjconfig.RuntimeStackInspection{Status: sjconfig.RuntimeStackStatusInvalidConfig, Code: "invalid_config/manifest", Reason: "内置运行组件清单无效。"}
	}
	return sjconfig.InspectRuntimeStackWithManifest(instance.DataDir, runtimeContentInstalled(instance.State), sjconfig.SteamInviteEnabled(instance.DataDir), manifest)
}

func (d *Driver) inspectManagedRuntimeStack(instance registry.Instance) sjconfig.RuntimeStackInspection {
	inspection := d.inspectRuntimeStack(instance)
	manifest, err := d.runtimeStackManifest()
	if err != nil {
		return inspection
	}
	return inspectManagedRuntimeControl(instance.DataDir, instance.State, inspection, manifest)
}
