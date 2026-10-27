package stardew_junimo

import (
	"context"
	"errors"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

type imageSMAPIStageFake struct {
	metadata   []paneldocker.RuntimeSMAPIMetadata
	reads      int
	installs   int
	stamps     int
	installErr error
}

func (f *imageSMAPIStageFake) RuntimeReadPinnedSMAPIMetadata(context.Context, string, string, string, string) (paneldocker.RuntimeSMAPIMetadata, error) {
	if f.reads >= len(f.metadata) {
		return paneldocker.RuntimeSMAPIMetadata{}, errors.New("unexpected metadata read")
	}
	value := f.metadata[f.reads]
	f.reads++
	return value, nil
}
func (f *imageSMAPIStageFake) RuntimeInstallImageSMAPI(context.Context, string, string, string, string) error {
	f.installs++
	return f.installErr
}
func (f *imageSMAPIStageFake) RuntimeCommitImageSMAPIStamp(context.Context, string, string, string, string, string) error {
	f.stamps++
	return nil
}

func TestJunimo140SMAPIStageCommitsOnlyVerifiedPayload(t *testing.T) {
	valid := paneldocker.RuntimeSMAPIMetadata{Present: true, RequiredFiles: true, ImageBuildID: junimo140SMAPIBuildID, CoreHashes: map[string]string{}}
	for key, value := range junimo140SMAPICoreHashes {
		valid.CoreHashes[key] = value
	}
	stamped := valid
	stamped.BuildID = junimo140SMAPIBuildID
	bad := valid
	bad.CoreHashes = map[string]string{}
	image := RuntimeUpdateSelectedImage{Digest: junimo140Digest}
	for _, test := range []struct {
		name             string
		metadata         []paneldocker.RuntimeSMAPIMetadata
		installErr       error
		wantError        bool
		installs, stamps int
	}{
		{"already verified", []paneldocker.RuntimeSMAPIMetadata{stamped}, nil, false, 0, 0},
		{"install and verify", []paneldocker.RuntimeSMAPIMetadata{{}, valid, stamped}, nil, false, 1, 1},
		{"bad payload", []paneldocker.RuntimeSMAPIMetadata{{}, bad}, nil, true, 1, 0},
		{"installer failed", []paneldocker.RuntimeSMAPIMetadata{{}}, errors.New("install failed"), true, 1, 0},
		{"stamp readback invalid", []paneldocker.RuntimeSMAPIMetadata{{}, valid, valid}, nil, true, 1, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake := &imageSMAPIStageFake{metadata: test.metadata, installErr: test.installErr}
			_, err := prepareJunimo140SMAPIStaging(context.Background(), fake, t.TempDir(), "staging", image)
			if (err != nil) != test.wantError || fake.installs != test.installs || fake.stamps != test.stamps {
				t.Fatalf("stage err=%v installs=%d stamps=%d", err, fake.installs, fake.stamps)
			}
		})
	}
}
