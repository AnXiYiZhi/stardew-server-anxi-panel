package stardew_junimo

import (
	"context"
	"errors"
	"strings"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

type junimo140InactiveImageDocker struct {
	junimo140GroupDocker
	metadata  paneldocker.RuntimeImageMetadata
	err       error
	inspected string
}

func (f *junimo140InactiveImageDocker) RuntimeImageInspect(_ context.Context, _, image string) (paneldocker.RuntimeImageMetadata, error) {
	f.inspected = image
	return f.metadata, f.err
}

func TestJunimo140ColdSourceBindsImageWithoutInventingContainer(t *testing.T) {
	for _, version := range []string{"1.5.0-preview.125", "1.5.0-preview.140"} {
		t.Run(version, func(t *testing.T) {
			original := junimo140WorldOriginal{}
			original.Current.Server.Tag = version
			digest := junimo140Digest
			if strings.HasSuffix(version, ".125") {
				digest = "sha256:10f438581d741fc146ce710cbe20099475ac68908e99f565cf449f0b8192ccf6"
			}
			fake := &junimo140InactiveImageDocker{metadata: paneldocker.RuntimeImageMetadata{ID: "sha256:" + strings.Repeat("b", 64), Digest: digest}}
			image := "sdvd/server:" + version
			if err := captureJunimo140InactiveServerImage(context.Background(), fake, t.TempDir(), image, &original); err != nil {
				t.Fatal(err)
			}
			if fake.inspected != image || original.Server.Image != image || original.Server.ImageID != fake.metadata.ID || original.Server.Digest != digest {
				t.Fatal("inactive source identity was not captured")
			}
			if original.ServerPresent || original.ServerWasRunning || original.ServerContainerID != "" || original.HasWorld {
				t.Fatal("capturing an image invented a service or world")
			}
		})
	}
}

func TestJunimo140ColdSourceRejectsUnavailableOrUnverifiedIdentity(t *testing.T) {
	for _, scenario := range []string{"missing image", "missing cache", "wrong digest", "invalid ID", "unsupported source"} {
		t.Run(scenario, func(t *testing.T) {
			original := junimo140WorldOriginal{}
			original.Current.Server.Tag = "1.5.0-preview.125"
			image := "sdvd/server:1.5.0-preview.125"
			fake := &junimo140InactiveImageDocker{metadata: paneldocker.RuntimeImageMetadata{ID: "sha256:" + strings.Repeat("b", 64), Digest: "sha256:10f438581d741fc146ce710cbe20099475ac68908e99f565cf449f0b8192ccf6"}}
			switch scenario {
			case "missing image":
				image = ""
			case "missing cache":
				fake.err = errors.New("missing")
			case "wrong digest":
				fake.metadata.Digest = junimo140Digest
			case "invalid ID":
				fake.metadata.ID = "mutable-tag"
			case "unsupported source":
				original.Current.Server.Tag = "custom"
			}
			if captureJunimo140InactiveServerImage(context.Background(), fake, t.TempDir(), image, &original) == nil {
				t.Fatal("unverified inactive source was accepted")
			}
			if original.Server != (RuntimeUpdateSelectedImage{}) || original.ServerPresent || original.ServerWasRunning {
				t.Fatal("rejected source changed the original snapshot")
			}
		})
	}
}

func TestJunimo140ExistingSourceKeepsCapturedContainerIdentity(t *testing.T) {
	original := junimo140WorldOriginal{ServerPresent: true, ServerWasRunning: true, ServerContainerID: strings.Repeat("c", 64), Server: RuntimeUpdateSelectedImage{Image: "captured", ImageID: "sha256:" + strings.Repeat("b", 64), Digest: junimo140Digest}}
	before := original
	fake := &junimo140InactiveImageDocker{err: errors.New("must not inspect alias")}
	if captureJunimo140InactiveServerImage(context.Background(), fake, t.TempDir(), "changed-alias", &original) != nil || original.Server != before.Server || original.ServerContainerID != before.ServerContainerID || !original.ServerWasRunning || fake.inspected != "" {
		t.Fatal("existing container identity was replaced by an alias")
	}
}
