package stardew_junimo

import (
	"context"
	"path/filepath"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

func TestJunimo140SMAPIIdentityRequiresStampAndEveryCoreHash(t *testing.T) {
	metadata := paneldocker.RuntimeSMAPIMetadata{Present: true, RequiredFiles: true, BuildID: junimo140SMAPIBuildID, ImageBuildID: junimo140SMAPIBuildID, CoreHashes: map[string]string{}}
	for name, hash := range junimo140SMAPICoreHashes {
		metadata.CoreHashes[name] = hash
	}
	if !junimo140SMAPIIdentityVerified(metadata) {
		t.Fatal("exact fixed image build rejected")
	}
	for name, hash := range metadata.CoreHashes {
		metadata.CoreHashes[name] = "corrupt"
		if junimo140SMAPIIdentityVerified(metadata) {
			t.Fatalf("stamp hid overwritten core file %s", name)
		}
		metadata.CoreHashes[name] = hash
	}
	metadata.BuildID = "4.5.2-original"
	if junimo140SMAPIIdentityVerified(metadata) {
		t.Fatal("original installer accepted as patched build")
	}
}

func TestJunimo140SMAPINeverOffersOriginalInstaller(t *testing.T) {
	driver, instance, fake := smapiDetectionFixture(t, "4.5.2")
	if err := sjconfig.UpdateEnvFile(filepath.Join(instance.DataDir, ".env"), map[string]string{"SERVER_IMAGE": "sdvd/server:1.5.0-preview.140@" + junimo140Digest}); err != nil {
		t.Fatal(err)
	}
	for _, metadata := range []paneldocker.RuntimeSMAPIMetadata{
		{},
		{Present: true, RequiredFiles: true, BuildID: junimo140SMAPIBuildID, ImageBuildID: junimo140SMAPIBuildID},
		{Present: true, RequiredFiles: true, BuildID: junimo140SMAPIBuildID, ImageBuildID: junimo140SMAPIBuildID, CoreHashes: junimo140SMAPICoreHashes},
	} {
		fake.metadata = metadata
		got, err := driver.InspectSMAPIUpdate(context.Background(), instance)
		if err != nil || got.Supported || got.Available {
			t.Fatalf("original installer offered for image-managed runtime: %+v err=%v", got, err)
		}
		if got.Current.BuildVerified != junimo140SMAPIIdentityVerified(metadata) {
			t.Fatal("build identity not reflected in current metadata")
		}
	}
}
