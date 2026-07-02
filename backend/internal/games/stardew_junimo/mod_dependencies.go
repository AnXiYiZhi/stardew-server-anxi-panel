package stardew_junimo

import (
	"strconv"
	"strings"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

const (
	modDependencyStatusSatisfied               = "satisfied"
	modDependencyStatusMissing                 = "missing"
	modDependencyStatusDisabled                = "disabled"
	modDependencyStatusVersionMismatch         = "version_mismatch"
	modDependencyStatusUnknownVersion          = "unknown_version"
	modDependencyStatusOptionalMissing         = "optional_missing"
	modDependencyStatusOptionalDisabled        = "optional_disabled"
	modDependencyStatusOptionalVersionMismatch = "optional_version_mismatch"
	modDependencyStatusOptionalUnknownVersion  = "optional_unknown_version"
)

func applyModDependencyStatus(mods []registry.ModInfo) []registry.ModInfo {
	installedByUniqueID := make(map[string]registry.ModInfo, len(mods))
	for _, mod := range mods {
		uniqueID := strings.TrimSpace(mod.UniqueID)
		if uniqueID == "" || mod.ParseError != "" {
			continue
		}
		installedByUniqueID[strings.ToLower(uniqueID)] = mod
	}

	for i := range mods {
		deps := mods[i].Dependencies
		for j := range deps {
			deps[j] = resolveModDependencyStatus(deps[j], installedByUniqueID)
		}
		mods[i].Dependencies = deps
	}
	return mods
}

func resolveModDependencyStatus(dep registry.ModDependency, installedByUniqueID map[string]registry.ModInfo) registry.ModDependency {
	dep.UniqueID = strings.TrimSpace(dep.UniqueID)
	dep.MinimumVersion = strings.TrimSpace(dep.MinimumVersion)
	dep.Installed = false
	dep.Enabled = false
	dep.InstalledVersion = ""
	dep.Satisfied = !dep.Required
	dep.Status = modDependencyStatusOptionalMissing
	if dep.Required {
		dep.Status = modDependencyStatusMissing
	}

	if dep.UniqueID == "" {
		return dep
	}
	match, ok := installedByUniqueID[strings.ToLower(dep.UniqueID)]
	if !ok {
		return dep
	}

	dep.Installed = true
	dep.Enabled = match.Enabled
	dep.InstalledVersion = strings.TrimSpace(match.Version)
	if !match.Enabled {
		dep.Satisfied = !dep.Required
		if dep.Required {
			dep.Status = modDependencyStatusDisabled
		} else {
			dep.Status = modDependencyStatusOptionalDisabled
		}
		return dep
	}

	if dep.MinimumVersion != "" {
		cmp, ok := compareModVersions(dep.InstalledVersion, dep.MinimumVersion)
		if !ok {
			dep.Satisfied = !dep.Required
			if dep.Required {
				dep.Status = modDependencyStatusUnknownVersion
			} else {
				dep.Status = modDependencyStatusOptionalUnknownVersion
			}
			return dep
		}
		if cmp < 0 {
			dep.Satisfied = !dep.Required
			if dep.Required {
				dep.Status = modDependencyStatusVersionMismatch
			} else {
				dep.Status = modDependencyStatusOptionalVersionMismatch
			}
			return dep
		}
	}

	dep.Satisfied = true
	dep.Status = modDependencyStatusSatisfied
	return dep
}

func compareModVersions(installed, minimum string) (int, bool) {
	installedParts, okInstalled := parseVersionParts(installed)
	minimumParts, okMinimum := parseVersionParts(minimum)
	if !okInstalled || !okMinimum {
		return 0, false
	}
	maxLen := len(installedParts)
	if len(minimumParts) > maxLen {
		maxLen = len(minimumParts)
	}
	for i := 0; i < maxLen; i++ {
		installedPart := 0
		if i < len(installedParts) {
			installedPart = installedParts[i]
		}
		minimumPart := 0
		if i < len(minimumParts) {
			minimumPart = minimumParts[i]
		}
		if installedPart < minimumPart {
			return -1, true
		}
		if installedPart > minimumPart {
			return 1, true
		}
	}
	return 0, true
}

func parseVersionParts(version string) ([]int, bool) {
	version = strings.TrimSpace(version)
	if version == "" {
		return nil, false
	}
	parts := []int{}
	start := -1
	flush := func(end int) bool {
		if start < 0 {
			return true
		}
		value, err := strconv.Atoi(version[start:end])
		if err != nil {
			return false
		}
		parts = append(parts, value)
		start = -1
		return true
	}
	for i, r := range version {
		if r >= '0' && r <= '9' {
			if start < 0 {
				start = i
			}
			continue
		}
		if !flush(i) {
			return nil, false
		}
	}
	if !flush(len(version)) {
		return nil, false
	}
	return parts, len(parts) > 0
}
