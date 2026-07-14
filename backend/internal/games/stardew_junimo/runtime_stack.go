package stardew_junimo

import (
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

// InspectRuntimeStack reports the immutable, read-only Junimo server and
// steam-auth-cn version-pair status for an instance.
func InspectRuntimeStack(dataDir, state string) sjconfig.RuntimeStackInspection {
	installed := state != storage.InstanceStateUninitialized && state != storage.InstanceStateAdminCreated
	return sjconfig.InspectRuntimeStack(dataDir, installed)
}
