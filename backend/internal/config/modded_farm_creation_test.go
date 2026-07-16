package config

import "testing"

func TestModdedFarmCreationDefaultsEnabled(t *testing.T) {
	t.Setenv("ENABLE_MODDED_FARM_CREATION", "")
	if !Load().EnableModdedFarmCreation {
		t.Fatal("EnableModdedFarmCreation = false, want true by default")
	}
}

func TestModdedFarmCreationCanBeExplicitlyDisabled(t *testing.T) {
	t.Setenv("ENABLE_MODDED_FARM_CREATION", "false")
	if Load().EnableModdedFarmCreation {
		t.Fatal("EnableModdedFarmCreation = true, want false for explicit override")
	}
}

func TestModdedFarmCreationInvalidValueUsesEnabledDefault(t *testing.T) {
	t.Setenv("ENABLE_MODDED_FARM_CREATION", "invalid")
	if !Load().EnableModdedFarmCreation {
		t.Fatal("EnableModdedFarmCreation = false, want enabled fallback for invalid value")
	}
}
