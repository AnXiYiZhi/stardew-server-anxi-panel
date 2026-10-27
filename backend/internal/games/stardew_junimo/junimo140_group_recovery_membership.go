package stardew_junimo

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

// A full-root restore removes .env before tar recreates it. Only an exact
// database member with a durable, fully backed-up restore intent may bypass
// current .env discovery. This grants no permission to restore bytes: the Docker
// restore still verifies the entire immutable archive before any purge.
func validateJunimo140RecoveryMembership(instances []storage.Instance, plan junimo140GroupPlan, txn junimo140GroupTransaction) error {
	if err := validateJunimo140GroupTransaction(plan, txn); err != nil {
		return err
	}
	members := make(map[string]int, len(plan.Members))
	for i, member := range plan.Members {
		members[member.ID] = i
	}
	seen := map[string]bool{}
	found := map[string]bool{}
	roots := []string{}
	rollback := txn.Phase == "rolling_back" || txn.Phase == "rollback_failed"
	for _, instance := range instances {
		if instance.DriverID != DriverID && instance.DataDir == "" {
			continue
		}
		if instance.ID == "" || seen[instance.ID] {
			return errors.New("shared recovery instance identity is ambiguous")
		}
		seen[instance.ID] = true
		if err := plainJunimoGroupRoot(instance.DataDir); err != nil {
			return err
		}
		for _, root := range roots {
			if junimoGroupPathContains(root, instance.DataDir) || junimoGroupPathContains(instance.DataDir, root) {
				return errors.New("shared recovery instance roots overlap")
			}
		}
		roots = append(roots, instance.DataDir)
		index, known := members[instance.ID]
		if known {
			member := plan.Members[index]
			if instance.DriverID != DriverID || !sameJunimoGroupPath(instance.DataDir, member.DataDir) {
				return errors.New("shared recovery member root or driver changed")
			}
			found[instance.ID] = true
			if rollback && txn.Worlds[index].RestoreIntent {
				continue
			}
		}
		if instance.DriverID != DriverID {
			continue
		}
		envPath := filepath.Join(instance.DataDir, ".env")
		info, err := os.Lstat(envPath)
		if errors.Is(err, os.ErrNotExist) && !known {
			continue
		}
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > 1024*1024 {
			return errors.New("shared recovery environment is unavailable or unsafe")
		}
		volume, err := GameDataVolumeName(instance.DataDir)
		if err != nil {
			return err
		}
		if !known {
			if volume == plan.SourceVolume || volume == plan.StagingVolume {
				return errors.New("a new world joined the shared game group during recovery")
			}
			continue
		}
		if volume != plan.SourceVolume && volume != plan.StagingVolume {
			return errors.New("shared recovery member game binding changed")
		}
		env, err := sjconfig.ReadEnvFile(envPath)
		if err != nil {
			return err
		}
		for key, expected := range map[string]string{"USER_ID": plan.UID, "GROUP_ID": plan.GID} {
			value := strings.TrimSpace(env[key])
			if value == "" {
				value = "1000"
			}
			number, err := strconv.ParseUint(value, 10, 32)
			if err != nil || number == 4294967295 || strconv.FormatUint(number, 10) != expected {
				return errors.New("shared recovery member ownership changed")
			}
		}
	}
	if len(found) != len(plan.Members) {
		return errors.New("a shared recovery member disappeared from the instance store")
	}
	return nil
}
