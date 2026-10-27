package stardew_junimo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

func TestJunimo140DisabledInviteKeepsCompatibleDeferredAuth(t *testing.T) {
	target, err := fixedFullStackJunimo140Target()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	before := "STEAM_INVITE_ENABLED=false\nSTEAM_SERVICE_IMAGE=anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2\n"
	if err := os.WriteFile(path, []byte(before), 0600); err != nil {
		t.Fatal(err)
	}
	selected := RuntimeUpdateSelectedPair{Server: RuntimeUpdateSelectedImage{Image: target.Server.Image}}
	if err := writeRuntimeTargetEnvAtomic(dir, target, selected, false); err != nil {
		t.Fatal(err)
	}
	fields, err := sjconfig.ReadEnvFile(path)
	if err != nil || sjconfig.SteamInviteEnabled(dir) || fields["STEAM_SERVICE_IMAGE"] != target.SteamAuth.Image || fields["STEAM_SERVICE_IMAGE_CANDIDATES"] != strings.Join(target.SteamAuth.TrustedCandidates, ",") {
		t.Fatal("disabled invite did not retain the deferred compatible Auth target", err)
	}
	if err := os.WriteFile(path, []byte(before), 0600); err != nil {
		t.Fatal(err)
	}
	target.SteamAuth = sjconfig.RuntimeStackManifestComponent{Tag: "1.5.0-anxi.2"}
	if err := writeRuntimeTargetEnvAtomic(dir, target, selected, false); err == nil {
		t.Fatal("incompatible deferred Auth target was accepted")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != before {
		t.Fatal("failed target changed the original configuration", err)
	}
}

type committedAuthReceiptDocker struct {
	*fullStackReceiptDocker
	server RuntimeUpdateSelectedImage
	auth   paneldocker.RuntimeImageMetadata
}

func (f *committedAuthReceiptDocker) RuntimeImageInspect(_ context.Context, _ string, ref string) (paneldocker.RuntimeImageMetadata, error) {
	if ref == f.server.Image {
		return paneldocker.RuntimeImageMetadata{ID: f.server.ImageID, Digest: f.server.Digest}, nil
	}
	return f.auth, nil
}

func TestFullStackCommittedCompatibilityAllowsVerifiedInviteToggle(t *testing.T) {
	driver, op, instance := fullStackRollbackReceiptFixture(t)
	plan := op.Groups[0].Plan
	txn, err := readJunimo140GroupTransaction(plan)
	if err != nil {
		t.Fatal(err)
	}
	readyJunimo140Transaction(&txn)
	for i := range txn.Worlds {
		txn.Worlds[i].UpdateIntent, txn.Worlds[i].Updated = true, true
	}
	txn.Phase = "committed"
	path, err := junimo140GroupTransactionPath(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := writePrivateRuntimeUpdateJSON(path, txn); err != nil {
		t.Fatal(err)
	}
	for _, member := range plan.Members {
		if err := installSMAPIMod(member.DataDir); err != nil {
			t.Fatal(err)
		}
		if err := writeRuntimeTargetEnvAtomic(member.DataDir, txn.Recommendation, txn.Target, false); err != nil {
			t.Fatal(err)
		}
		if err := switchGameDataVolumeAtomic(member.DataDir, plan.StagingVolume); err != nil {
			t.Fatal(err)
		}
	}
	fake := &committedAuthReceiptDocker{fullStackReceiptDocker: driver.docker.(*fullStackReceiptDocker), server: txn.Target.Server,
		auth: paneldocker.RuntimeImageMetadata{ID: "sha256:" + strings.Repeat("e", 64), Digest: op.Target.SteamAuth.Digests[op.Target.SteamAuth.Image]}}
	driver.docker = fake
	if err := driver.writeFullStackCompatibility(op, 0); err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []bool{false, true, false} {
		value := "false"
		if enabled {
			value = "true"
		}
		if err := sjconfig.UpdateEnvFile(filepath.Join(instance.DataDir, ".env"), map[string]string{"STEAM_INVITE_ENABLED": value}); err != nil {
			t.Fatal(err)
		}
		if !driver.acceptsFullStackCompatibility(instance) || driver.requireCurrentRuntimeStack(instance) != nil {
			t.Fatalf("valid optional Auth toggle invalidated the committed migration: enabled=%v", enabled)
		}
		if enabled && driver.EffectiveRuntimeInspection(instance).Current.SteamAuth.Tag != sjconfig.Junimo140SteamAuthTag {
			t.Fatal("effective inspection hid newly enabled Auth")
		}
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(instance.DataDir, ".env"), map[string]string{"STEAM_INVITE_ENABLED": "true"}); err != nil {
		t.Fatal(err)
	}
	fake.auth.Digest = "sha256:" + strings.Repeat("f", 64)
	if driver.acceptsFullStackCompatibility(instance) {
		t.Fatal("changed Auth content inherited committed compatibility")
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(instance.DataDir, ".env"), map[string]string{"STEAM_SERVICE_IMAGE": "anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2"}); err != nil {
		t.Fatal(err)
	}
	if driver.acceptsFullStackCompatibility(instance) {
		t.Fatal("legacy Auth inherited committed compatibility")
	}
}
