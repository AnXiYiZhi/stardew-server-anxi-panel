package updater

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

func testPanelPin(image string) PanelTarget {
	return PanelTarget{Version: "0.1.15", Image: image, ImageID: "sha256:" + strings.Repeat("f", 64), Revision: strings.Repeat("a", 40), BuildDate: "2026-10-16T00:00:00Z"}
}
func pinMetadata(pin PanelTarget) string {
	raw, _ := json.Marshal([]map[string]any{{"Id": pin.ImageID, "Config": map[string]any{"Labels": map[string]string{"org.opencontainers.image.version": pin.Version, "org.opencontainers.image.revision": pin.Revision, "org.opencontainers.image.created": pin.BuildDate}}}})
	return string(raw)
}

type pinServiceRuntime struct {
	*fakeRuntime
	target   PanelTarget
	lookups  []string
	tagMoved bool
}

func (f *pinServiceRuntime) InspectPanelTarget(_ context.Context, image, version string) (PanelTarget, error) {
	f.lookups = append(f.lookups, image)
	if image != f.target.Image && image != f.target.ImageID || version != f.target.Version {
		return PanelTarget{}, errors.New("target mismatch")
	}
	pin := f.target
	if f.tagMoved && image == pin.Image {
		pin.ImageID = "sha256:" + strings.Repeat("e", 64)
	}
	pin.Image = image
	return pin, nil
}

func TestPinnedCurrentPanelUsesRunningContentWhenTagMoves(t *testing.T) {
	service, runtime := newApplyServiceTest(t, &fakeDatabaseBackupper{})
	pin := testPanelPin("ghcr.io/anxiyizhi/stardew-server-anxi-panel:0.1.15")
	runtime.info.Image, runtime.info.ImageID = pin.Image, pin.ImageID
	inspector := &pinServiceRuntime{fakeRuntime: runtime, target: pin, tagMoved: true}
	service.docker = inspector
	current, err := service.PreparedPanelTarget(context.Background(), pin.Version, pin.Version)
	if err != nil || !current.SameContent(pin) || len(inspector.lookups) != 1 || inspector.lookups[0] != pin.ImageID {
		t.Fatal("retagging replaced the identity of the running Panel", err, inspector.lookups)
	}
	runtime.info.ImageID = ""
	if _, err := service.PreparedPanelTarget(context.Background(), pin.Version, pin.Version); err == nil {
		t.Fatal("unknown running identity fell back to the mutable tag")
	}
	alias := pin
	alias.Image = pin.ImageID
	if !pin.SameContent(alias) {
		t.Fatal("helper's content-addressed alias changed confirmed content")
	}
	alias.BuildDate = "2026-10-17T00:00:00Z"
	if pin.SameContent(alias) {
		t.Fatal("different build inherited the target identity")
	}
}

func TestPinnedPanelApplyKeepsOneTaskAcrossTerminalReplays(t *testing.T) {
	db := &fakeDatabaseBackupper{}
	service, runtime := newApplyServiceTest(t, db)
	pin := testPanelPin("anxiyizhi/stardew-server-anxi-panel:0.1.15")
	service.docker = &pinServiceRuntime{fakeRuntime: runtime, target: pin}
	if err := service.store.Write(DryRunStatus{Phase: "succeeded", TargetVersion: pin.Version, TargetImage: pin.Image}); err != nil {
		t.Fatal(err)
	}
	id := "stack_" + strings.Repeat("b", 32)
	first, err := service.StartApplyPinned(context.Background(), "0.1.14", pin, id)
	if err != nil {
		t.Fatal(err)
	}
	first.Phase = PhaseSucceeded
	if err := service.applyStore.Write(first); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 3; n++ {
		again, err := service.StartApplyPinned(context.Background(), "0.1.14", pin, id)
		if err != nil || again.UpdateID != first.UpdateID {
			t.Fatal("terminal replay created a new Panel operation", err)
		}
	}
	if db.calls != 1 || len(runtime.applySpecs) != 1 {
		t.Fatal("confirmation replay repeated backup or helper dispatch")
	}
	pin.ImageID = "sha256:" + strings.Repeat("e", 64)
	if _, err := service.StartApplyPinned(context.Background(), "0.1.14", pin, id); err == nil {
		t.Fatal("same authorization changed target")
	}
}

type pinApplyExecutor struct {
	*applyScenarioExecutor
	pin                                   PanelTarget
	missing, changedOCI, changedContainer bool
}

func (e *pinApplyExecutor) Run(ctx context.Context, args ...string) error {
	if containsArg(args, "up") {
		env, err := os.ReadFile(argAfter(args, "--env-file"))
		if err == nil && strings.Contains(string(env), "PANEL_IMAGE="+e.pin.ImageID) {
			e.calls = append(e.calls, append([]string(nil), args...))
			e.currentNew = true
			return nil
		}
	}
	return e.applyScenarioExecutor.Run(ctx, args...)
}
func (e *pinApplyExecutor) Output(ctx context.Context, args ...string) (string, error) {
	if len(args) == 3 && args[0] == "image" && args[1] == "inspect" && args[2] == e.pin.ImageID {
		e.calls = append(e.calls, append([]string(nil), args...))
		if e.missing {
			return "", errors.New("cached image missing")
		}
		pin := e.pin
		if e.changedOCI {
			pin.Revision = strings.Repeat("b", 40)
		}
		return pinMetadata(pin), nil
	}
	if len(args) > 2 && args[0] == "inspect" && args[2] == "{{.Image}}" {
		if e.changedContainer {
			return "sha256:" + strings.Repeat("c", 64), nil
		}
		return e.pin.ImageID, nil
	}
	return e.applyScenarioExecutor.Output(ctx, args...)
}

func TestPinnedPanelHelperUsesConfirmedContentAndNeverRepullsTags(t *testing.T) {
	for _, scenario := range []string{"success", "missing cache", "changed OCI", "wrong container", "unhealthy"} {
		t.Run(scenario, func(t *testing.T) {
			base := &applyScenarioExecutor{}
			opts, store, _, _ := prepareApplyTest(t, base)
			pin := testPanelPin(base.newImage)
			executor := &pinApplyExecutor{applyScenarioExecutor: base, pin: pin, missing: scenario == "missing cache", changedOCI: scenario == "changed OCI", changedContainer: scenario == "wrong container"}
			base.unhealthy = scenario == "unhealthy"
			opts.Executor = executor
			status, err := store.Read()
			if err != nil {
				t.Fatal(err)
			}
			status.OperationID, status.PinnedTarget = "stack_"+strings.Repeat("b", 32), &pin
			if err := store.Write(status); err != nil {
				t.Fatal(err)
			}
			err = RunApply(context.Background(), opts)
			final, readErr := store.Read()
			if readErr != nil {
				t.Fatal(readErr)
			}
			if scenario == "success" {
				if err != nil || final.Phase != PhaseSucceeded || final.SelectedDigest != pin.ImageID {
					t.Fatal("pinned helper did not use the confirmed image", err)
				}
			} else if err == nil || final.Phase != PhaseFailedRolledBack {
				t.Fatal("pinned failure did not restore the old Panel", err, final.Phase)
			}
			for _, call := range base.calls {
				if len(call) > 0 && call[0] == "pull" {
					t.Fatal("helper repulled a mutable tag after authorization")
				}
				if containsArg(call, "up") && argAfter(call, "--pull") != "never" {
					t.Fatal("pinned deployment did not use cached immutable content")
				}
				if (scenario == "missing cache" || scenario == "changed OCI") && (containsArg(call, "up") || len(call) > 0 && call[0] == "stop") {
					t.Fatal("failed preflight stopped the original Panel")
				}
			}
		})
	}
}

func TestPinnedPanelOCIRequiresExactVersionRevisionAndBuildDate(t *testing.T) {
	pin := testPanelPin("anxiyizhi/stardew-server-anxi-panel:0.1.15")
	if observed, err := parsePinnedPanelTarget([]byte(pinMetadata(pin)), pin.Image, pin.Version); err != nil || observed != pin {
		t.Fatal(err)
	}
	for _, field := range []string{"version", "revision", "date", "image"} {
		invalid := pin
		switch field {
		case "version":
			invalid.Version = "0.1.16"
		case "revision":
			invalid.Revision = "short"
		case "date":
			invalid.BuildDate = "today"
		case "image":
			invalid.ImageID = "sha256:short"
		}
		if _, err := parsePinnedPanelTarget([]byte(pinMetadata(invalid)), pin.Image, pin.Version); err == nil {
			t.Fatal("invalid OCI identity accepted", field)
		}
	}
}
