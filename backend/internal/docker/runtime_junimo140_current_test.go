package docker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCurrentJunimo140ReadUsesLiveScopeAndNoCopy(t *testing.T) {
	for _, scenario := range []string{"known live", "foreign live", "staging live"} {
		t.Run(scenario, func(t *testing.T) {
			f := newCurrentReaderFixture(t, scenario)
			var data []byte
			var err error
			if scenario == "staging live" {
				data, err = f.client.RuntimeReadJunimo140GameEvidence(context.Background(), f.dir, f.volume, "sdvd/server:1.5.0-preview.140", f.image, "game")
			} else {
				data, err = f.client.RuntimeReadCurrentJunimo140GameEvidence(context.Background(), f.dir, f.volume, "sdvd/server:1.5.0-preview.140", f.image, "game", []string{"world-a"})
			}
			if scenario != "known live" {
				if err == nil {
					t.Fatal("unsafe reader scope accepted")
				}
				if _, err := os.Stat(f.args); !os.IsNotExist(err) {
					t.Fatal("rejected scope created a helper")
				}
				return
			}
			if err != nil || string(data) != "fixed-read-evidence" {
				t.Fatal("known live group read rejected", err)
			}
			raw, err := os.ReadFile(f.args)
			if err != nil {
				t.Fatal(err)
			}
			for _, required := range []string{"--user\n0:0", "--network\nnone", "--read-only", "--cap-drop\nALL", "--cap-add\nDAC_READ_SEARCH", "--pull\nnever", "type=volume,src=" + f.volume + ",dst=/game,readonly,volume-nocopy"} {
				if !strings.Contains(string(raw), required) {
					t.Fatalf("read helper lost %q", required)
				}
			}
			events, _ := os.ReadFile(f.events)
			if string(events) != "CREATE\nSTART\nREMOVE\n" {
				t.Fatal("reader was replayed or not cleaned", string(events))
			}
			if _, err := os.Stat(f.state); !os.IsNotExist(err) {
				t.Fatal("owned helper survived successful read")
			}
		})
	}
}

func TestCurrentJunimo140EveryReadRejectsMissingScope(t *testing.T) {
	client := NewClient(Options{DockerPath: filepath.Join(t.TempDir(), "must-not-run")})
	ctx := context.Background()
	id := "sha256:" + strings.Repeat("a", 64)
	if _, err := client.RuntimeReadCurrentJunimo140GameEvidence(ctx, ".", "game", "sdvd/server:1.5.0-preview.140", id, "game", nil); err == nil {
		t.Fatal("evidence read missing scope")
	}
	if _, err := client.RuntimeReadCurrentJunimo140GameEvidence(ctx, ".", "game", "sdvd/server:1.5.0-preview.140", id, RuntimeCurrentJunimo140GalaxyPairEvidence, nil); err == nil || err.Error() != "current game scope missing" {
		t.Fatal("Galaxy evidence must use a current group scope", err)
	}
	if _, err := client.RuntimeReadJunimo140GameEvidence(ctx, ".", "game", "sdvd/server:1.5.0-preview.140", id, RuntimeCurrentJunimo140GalaxyPairEvidence); err == nil || err.Error() != "invalid fixed game evidence kind" {
		t.Fatal("Galaxy evidence must not be available to staging", err)
	}
	if _, err := client.RuntimeVerifyCurrentJunimo140GameFiles(ctx, ".", "game", "sdvd/server:1.5.0-preview.140", id, "sha256", nil, nil); err == nil {
		t.Fatal("checksum read missing scope")
	}
	if _, err := client.RuntimeInspectCurrentJunimo140SDK(ctx, ".", "game", "sdvd/server:1.5.0-preview.140", id, nil); err == nil {
		t.Fatal("SDK read missing scope")
	}
	if _, err := client.RuntimeReadCurrentPinnedSMAPIMetadata(ctx, ".", "game", "sdvd/server:1.5.0-preview.140", id, nil); err == nil {
		t.Fatal("SMAPI read missing scope")
	}
}

func TestCurrentJunimo140HelperMountKeepsOriginalStagingContract(t *testing.T) {
	volume := "world-a_anxi-smapi-update-" + strings.Repeat("a", 24)
	if strings.Contains(junimo140ReadMount(volume, nil), "volume-nocopy") {
		t.Fatal("original staging read mount changed")
	}
	if !strings.HasSuffix(junimo140ReadMount("existing-game", []string{"world-a"}), ",readonly,volume-nocopy") {
		t.Fatal("current read could populate the source")
	}
	args, err := junimo140ReadHelperArgs("existing-game", "sdk-inspect", []string{"world-a"})
	if err != nil || !strings.Contains(strings.Join(args, " "), "--user 0:0") {
		t.Fatal("current helper lost numeric identity", err)
	}
}
