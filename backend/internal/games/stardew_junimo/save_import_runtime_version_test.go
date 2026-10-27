package stardew_junimo

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

func TestImportRunningJunimoMatchesConfiguredQualifiedVersion(t *testing.T) {
	for _, tc := range []struct {
		name, configured, running string
		wantOK                    bool
	}{
		{"125", "1.5.0-preview.125", "1.5.0-preview.125", true},
		{"140", "1.5.0-preview.140", "1.5.0-preview.140", true},
		{"140_with_old_runtime", "1.5.0-preview.140", "1.5.0-preview.125", false},
		{"125_with_other_runtime", "1.5.0-preview.125", "1.5.0-preview.140", false},
		{"unqualified", "1.5.0-preview.141", "1.5.0-preview.141", false},
		{"missing_config", "", "1.5.0-preview.140", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.configured != "" {
				if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("IMAGE_VERSION="+tc.configured+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			docker := &fakeConsoleDocker{execFunc: func(_ context.Context, _ string, _ string, _ string, args ...string) (paneldocker.CommandResult, error) {
				calls++
				return paneldocker.CommandResult{Stdout: `{"Version":"` + tc.running + `"}`}, nil
			}}
			err := verifyRunningJunimoVersion(context.Background(), docker, dir)
			if (err == nil) != tc.wantOK {
				t.Fatalf("configured=%s running=%s err=%v", tc.configured, tc.running, err)
			}
			if tc.configured == "" || tc.configured == "1.5.0-preview.141" {
				if calls != 0 {
					t.Fatal("unsupported configuration reached runtime command")
				}
			}
		})
	}
}
