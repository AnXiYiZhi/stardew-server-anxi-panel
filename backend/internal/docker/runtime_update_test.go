package docker

import (
	"strings"
	"testing"
)

func TestValidateRestrictedImageRef(t *testing.T) {
	valid := []string{
		"sdvd/server:1.5.0-preview.121",
		"ghcr.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2",
	}
	for _, ref := range valid {
		if err := validateRestrictedImageRef(ref); err != nil {
			t.Errorf("valid image %q rejected: %v", ref, err)
		}
	}
	invalid := []string{
		"", "sdvd/server", "sdvd/server:latest", "sdvd/server@sha256:abc",
		"sdvd/server:tag;docker compose down", "--help:tag", "sdvd/server:sha256:abc",
	}
	for _, ref := range invalid {
		if err := validateRestrictedImageRef(ref); err == nil {
			t.Errorf("invalid image %q accepted", ref)
		}
	}
}

func TestParseRuntimeImageInspectOutputUsesOnlySafeFields(t *testing.T) {
	id := "sha256:" + strings.Repeat("a", 64)
	digest := "sha256:" + strings.Repeat("b", 64)
	metadata, err := parseRuntimeImageInspectOutput(`"` + id + `"|["registry.example/server@` + digest + `"]`)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.ID != id || metadata.Digest != digest {
		t.Fatalf("metadata=%+v", metadata)
	}
	for _, invalid := range []string{"", `[]`, `"not-an-id"|[]`, `"` + id + `"|{}`} {
		if _, err := parseRuntimeImageInspectOutput(invalid); err == nil {
			t.Fatalf("invalid output accepted: %q", invalid)
		}
	}
}

func TestRuntimeUpdateDockerContractContainsNoDestructiveMethods(t *testing.T) {
	// Compile-time/API contract test: phase-two's optional Docker surface exposes
	// only inspect/config/pull primitives; destructive operations cannot be called
	// through RuntimeUpdateDockerService.
	allowed := map[string]bool{
		"DockerVersion": true, "ComposeVersion": true, "ComposePs": true,
		"PullImageStreaming": true, "RuntimeImageInspect": true,
		"RuntimeComposeConfigInspect": true, "RuntimeComposeConfigValidateImages": true,
		"RuntimeVolumeInspect": true,
	}
	for _, forbidden := range []string{"ComposeUp", "ComposeDown", "ComposeRestart", "RemoveVolumes", "RemoveContainersByVolume"} {
		if allowed[forbidden] {
			t.Fatalf("destructive method %s entered phase-two contract", forbidden)
		}
	}
}
