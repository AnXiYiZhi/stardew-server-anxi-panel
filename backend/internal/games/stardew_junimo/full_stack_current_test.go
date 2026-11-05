package stardew_junimo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	appconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/updater"
)

type currentRuntimeTestStore struct {
	*storage.Store
	instances []storage.Instance
}

func (s *currentRuntimeTestStore) ListInstances(context.Context) ([]storage.Instance, error) {
	return append([]storage.Instance(nil), s.instances...), nil
}
func (s *currentRuntimeTestStore) GetInstance(_ context.Context, id string) (storage.Instance, error) {
	for _, instance := range s.instances {
		if instance.ID == id {
			return instance, nil
		}
	}
	return storage.Instance{}, storage.ErrNotFound
}

type currentRuntimeTestDocker struct {
	*fakeDocker
	services                   []paneldocker.RuntimeJunimo140ServiceSnapshot
	mod                        map[string]string
	failHolder, failSDK, drift bool
	reads                      int
}

func (f *currentRuntimeTestDocker) RuntimeSnapshotJunimo140Services(context.Context, string, []paneldocker.RuntimeJunimo140WorldScope, []string) ([]paneldocker.RuntimeJunimo140ServiceSnapshot, error) {
	f.reads++
	result := append([]paneldocker.RuntimeJunimo140ServiceSnapshot(nil), f.services...)
	if f.drift && f.reads > 1 {
		result[0].ContainerID = strings.Repeat("9", 64)
	}
	return result, nil
}
func (f *currentRuntimeTestDocker) RuntimeAssertJunimo140GameGroup(context.Context, string, string, []string, bool) error {
	if f.failHolder {
		return errors.New("unknown writer")
	}
	return nil
}
func (f *currentRuntimeTestDocker) RuntimeReadJunimo140ModHashes(context.Context, string, string, string) (map[string]string, error) {
	return f.mod, nil
}
func (f *currentRuntimeTestDocker) RuntimeInspectJunimo140SDK(ctx context.Context, dir, volume, image, id string) (paneldocker.RuntimeGameFileStats, error) {
	if f.failSDK {
		return paneldocker.RuntimeGameFileStats{}, errors.New("invalid ELF")
	}
	return f.fakeDocker.RuntimeInspectJunimo140SDK(ctx, dir, volume, image, id)
}
func (f *currentRuntimeTestDocker) RuntimeServerHealth(context.Context, string, string) error {
	return nil
}
func (f *currentRuntimeTestDocker) RuntimeSteamAuthHealth(context.Context, string, string) (paneldocker.RuntimeAuthServiceHealth, error) {
	return paneldocker.RuntimeAuthServiceHealth{LoggedIn: true}, nil
}

func (f *currentRuntimeTestDocker) RuntimeReadCurrentPinnedSMAPIMetadata(ctx context.Context, dir, volume, image, id string, projects []string) (paneldocker.RuntimeSMAPIMetadata, error) {
	if err := f.RuntimeAssertJunimo140GameGroup(ctx, dir, volume, projects, false); err != nil {
		return paneldocker.RuntimeSMAPIMetadata{}, err
	}
	return f.fakeDocker.RuntimeReadPinnedSMAPIMetadata(ctx, dir, volume, image, id)
}
func (f *currentRuntimeTestDocker) RuntimeReadCurrentJunimo140GameEvidence(ctx context.Context, dir, volume, image, id, kind string, projects []string) ([]byte, error) {
	if err := f.RuntimeAssertJunimo140GameGroup(ctx, dir, volume, projects, false); err != nil {
		return nil, err
	}
	return f.fakeDocker.RuntimeReadJunimo140GameEvidence(ctx, dir, volume, image, id, kind)
}
func (f *currentRuntimeTestDocker) RuntimeVerifyCurrentJunimo140GameFiles(ctx context.Context, dir, volume, image, id, algorithm string, files map[string]string, projects []string) (paneldocker.RuntimeGameFileStats, error) {
	if err := f.RuntimeAssertJunimo140GameGroup(ctx, dir, volume, projects, false); err != nil {
		return paneldocker.RuntimeGameFileStats{}, err
	}
	return f.fakeDocker.RuntimeVerifyJunimo140GameFiles(ctx, dir, volume, image, id, algorithm, files)
}
func (f *currentRuntimeTestDocker) RuntimeInspectCurrentJunimo140SDK(ctx context.Context, dir, volume, image, id string, projects []string) (paneldocker.RuntimeGameFileStats, error) {
	if err := f.RuntimeAssertJunimo140GameGroup(ctx, dir, volume, projects, false); err != nil {
		return paneldocker.RuntimeGameFileStats{}, err
	}
	return f.RuntimeInspectJunimo140SDK(ctx, dir, volume, image, id)
}

func currentRuntimeFixture(t *testing.T, sdkBuild string) (*Driver, *currentRuntimeTestDocker, *currentRuntimeTestStore, updater.PanelTarget) {
	t.Helper()
	root := t.TempDir()
	db, err := storage.Open(context.Background(), appconfig.Config{DataDir: root, DBPath: filepath.Join(root, "panel.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	store := &currentRuntimeTestStore{Store: db}
	base := &fakeDocker{}
	manifestID := "6403079453713498174"
	if sdkBuild == "25329704" {
		manifestID = "4559160656493359681"
	}
	completeJunimo140GameEvidence(t, base, installedSDKFixture(sdkBuild, manifestID))
	fake := &currentRuntimeTestDocker{fakeDocker: base, mod: map[string]string{}}
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a", "b"} {
		dir := filepath.Join(root, "world-"+id)
		for _, path := range []string{dir, smapiModDir(dir), junimoServerModDir(dir)} {
			if err := os.MkdirAll(path, 0700); err != nil {
				t.Fatal(err)
			}
		}
		files := map[string][]byte{
			filepath.Join(dir, ".env"):                                      []byte("IMAGE_VERSION=1.5.0-preview.140\nSERVER_IMAGE=sdvd/server:1.5.0-preview.140\nSERVER_IMAGE_CANDIDATES=sdvd/server:1.5.0-preview.140\nSTEAM_INVITE_ENABLED=false\nGAME_DATA_VOLUME=shared-current-game\n"),
			filepath.Join(dir, "docker-compose.yml"):                        []byte(junimoComposeTemplate),
			filepath.Join(smapiModDir(dir), "manifest.json"):                []byte(`{"Version":"` + manifest.Control.Version + `"}`),
			filepath.Join(smapiModDir(dir), "StardewAnxiPanel.Control.dll"): smapiModDLL,
			filepath.Join(junimoServerModDir(dir), "manifest.json"):         []byte(`{"Version":"1.5.0-preview.140"}`),
			filepath.Join(junimoServerModDir(dir), "JunimoServer.dll"):      []byte("test-only fixed image Mod payload"),
		}
		for path, raw := range files {
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
		}
		for _, name := range []string{"manifest.json", "JunimoServer.dll"} {
			fake.mod[name], err = runtimeRecoveryFileSHA256(filepath.Join(junimoServerModDir(dir), name))
			if err != nil {
				t.Fatal(err)
			}
		}
		store.instances = append(store.instances, storage.Instance{ID: id, DriverID: DriverID, DataDir: dir, State: storage.InstanceStateStopped})
		fake.services = append(fake.services, paneldocker.RuntimeJunimo140ServiceSnapshot{Project: "world-" + id, Service: "server", GameVolume: "shared-current-game", RuntimeServiceMetadata: paneldocker.RuntimeServiceMetadata{ContainerID: strings.Repeat(id, 64), Image: "sdvd/server:1.5.0-preview.140", ImageID: "sha256:" + strings.Repeat("f", 64), State: "exited"}})
	}
	pin := fullStackTestPanel("0.8.1", "c")
	driver := NewWithOptions(fake, nil, jobs.NewManager(db, nil), store, DriverOptions{PanelVersion: pin.Version, PanelIdentity: &pin, ContainerDataDir: root, HostDataDir: root})
	return driver, fake, store, pin
}

func TestCurrentRuntimeNoopFullEvidenceAndZeroGameMutation(t *testing.T) {
	for _, sdk := range []string{"20939719", "25329704"} {
		t.Run(sdk, func(t *testing.T) {
			d, f, s, pin := currentRuntimeFixture(t, sdk)
			_, members, err := d.currentRuntimeMembers(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			services := append([]paneldocker.RuntimeJunimo140ServiceSnapshot(nil), f.services...)
			apply := updater.ApplyStatus{UpdateID: "actual-apply", Phase: updater.PhaseSucceeded, ToVersion: pin.Version, SelectedDigest: pin.ImageID, CleanupCompleted: true}
			full, err := d.inspectCurrentFullStack(context.Background(), pin, apply)
			if err != nil {
				t.Fatal(err)
			}
			if full.Phase != "not_needed" || full.RuntimeRequired || len(full.Instances) != 2 || full.CurrentRuntimeVerified.UpdateID != apply.UpdateID {
				t.Fatal("incomplete no-op", full)
			}
			apply.FullStack = &full
			if d.HasCurrentRuntimeNoop(context.Background(), apply) {
				t.Fatal("a persisted receipt alone established process trust")
			}
			// Model the successful CAS publication; only the coordinator installs this trust.
			d.currentRuntimeVerified.Store(full.CurrentRuntimeVerified)
			if !d.HasCurrentRuntimeNoop(context.Background(), apply) {
				t.Fatal("current receipt rejected")
			}
			_, after, err := d.currentRuntimeMembers(context.Background())
			if err != nil || !reflect.DeepEqual(members, after) || !reflect.DeepEqual(services, f.services) {
				t.Fatal("inspection changed runtime")
			}
			if len(f.gameDownloadMarkers) != 0 || f.installVolumeClones != 0 || f.installVolumeCreates != 0 || len(f.pulledImages) != 0 {
				t.Fatal("inspection called a game mutator")
			}
			for _, instance := range s.instances {
				if _, err := os.Stat(junimo140GroupLeasePath(instance.DataDir)); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("inspection claimed a transaction")
				}
			}
			reads := f.reads
			for n := 0; n < 3; n++ {
				if !d.HasCurrentRuntimeNoop(context.Background(), apply) {
					t.Fatal("receipt changed")
				}
			}
			if f.reads != reads {
				t.Fatal("GET-style receipt validation started Docker helpers")
			}
		})
	}
}

func TestCurrentRuntimeNoopRejectsIncompleteGroupsAndRecovery(t *testing.T) {
	for _, scenario := range []string{"legacy peer", "sdk corrupt", "smapi corrupt", "mod corrupt", "unknown holder", "container changed", "recovery journal", "opaque status", "control corrupt", "new preflight", "manual recovery"} {
		t.Run(scenario, func(t *testing.T) {
			d, f, s, pin := currentRuntimeFixture(t, "25329704")
			dir := s.instances[1].DataDir
			switch scenario {
			case "legacy peer":
				raw, _ := os.ReadFile(filepath.Join(dir, ".env"))
				_ = os.WriteFile(filepath.Join(dir, ".env"), []byte(strings.ReplaceAll(string(raw), "preview.140", "preview.125")), 0600)
			case "sdk corrupt":
				f.failSDK = true
			case "smapi corrupt":
				f.imageSMAPICoreMismatch = true
			case "mod corrupt":
				_ = os.WriteFile(filepath.Join(junimoServerModDir(dir), "JunimoServer.dll"), []byte("corrupt"), 0600)
			case "unknown holder":
				f.failHolder = true
			case "container changed":
				f.drift = true
			case "recovery journal":
				_ = os.MkdirAll(filepath.Dir(junimo140GroupLeasePath(dir)), 0700)
				_ = os.WriteFile(junimo140GroupLeasePath(dir), []byte("opaque"), 0600)
			case "opaque status":
				if err := writeRuntimeUpdateApplyStatus(dir, RuntimeUpdateApplyStatus{Phase: "unknown"}); err != nil {
					t.Fatal(err)
				}
			case "control corrupt":
				_ = os.WriteFile(filepath.Join(smapiModDir(dir), "StardewAnxiPanel.Control.dll"), []byte("corrupt"), 0600)
			case "new preflight", "manual recovery":
				target, _ := fixedFullStackJunimo140Target()
				op := fullStackOperation{Schema: 1, Target: target, Public: FullStackUpgradeStatus{OperationID: "stack_" + strings.Repeat("a", 32), ActorID: 41, Panel: pin, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour), Phase: "awaiting_confirmation"}}
				if scenario == "manual recovery" {
					now := time.Now()
					op.Public.AuthorizedAt = &now
					op.Public.Phase = "manual_action"
				}
				op.Public.PlanSHA256 = op.digest()
				if err := d.writeFullStackOperation(op); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := d.inspectCurrentFullStack(context.Background(), pin, updater.ApplyStatus{UpdateID: "actual-apply"}); err == nil {
				t.Fatal("incomplete evidence became no-op success")
			}
			if len(f.gameDownloadMarkers) != 0 || f.installVolumeClones != 0 || f.installVolumeCreates != 0 {
				t.Fatal("rejected inspection changed game")
			}
		})
	}
}

type currentRuntimeRecordingPanel struct {
	fullStackPanelFixture
	records int
}

func (f *currentRuntimeRecordingPanel) RecordRuntimeNoop(_ context.Context, expected updater.ApplyStatus, pin updater.PanelTarget, full updater.FullStackStatus) (bool, error) {
	if expected.UpdateID != f.status.UpdateID || !pin.SameContent(f.current) {
		return false, nil
	}
	f.records++
	f.status.FullStack = &full
	return true, nil
}

func TestCurrentRuntimeNoopCoordinatorRestartRechecksPayloadAndIdentity(t *testing.T) {
	d, f, store, pin := currentRuntimeFixture(t, "25329704")
	panel := &currentRuntimeRecordingPanel{fullStackPanelFixture: fullStackPanelFixture{current: pin, status: updater.ApplyStatus{UpdateID: "same-apply", Phase: updater.PhaseSucceeded, ToVersion: pin.Version, SelectedDigest: pin.ImageID, CleanupCompleted: true}}}
	attempts := currentRuntimeNoopAttempts{}
	d.reconcileCurrentRuntimeNoop(context.Background(), panel, &attempts)
	if panel.records != 1 || !d.HasCurrentRuntimeNoop(context.Background(), panel.status) {
		t.Fatal("coordinator did not publish exact no-op")
	}
	reads := f.reads
	for n := 0; n < 4; n++ {
		d.reconcileCurrentRuntimeNoop(context.Background(), panel, &attempts)
	}
	if panel.records != 1 || f.reads != reads {
		t.Fatal("same process repeated full inspection")
	}
	for _, scenario := range []string{"sdk drift", "container drift", "unchanged"} {
		t.Run(scenario, func(t *testing.T) {
			restarted := NewWithOptions(f, nil, d.jobs, store, DriverOptions{PanelVersion: pin.Version, PanelIdentity: &pin, ContainerDataDir: d.containerDataDir, HostDataDir: d.hostDataDir})
			if restarted.HasCurrentRuntimeNoop(context.Background(), panel.status) {
				t.Fatal("restart trusted persisted proof without strong inspection")
			}
			originalID := f.services[0].ImageID
			if scenario == "sdk drift" {
				f.failSDK = true
			}
			if scenario == "container drift" {
				f.services[0].ImageID = "sha256:" + strings.Repeat("a", 64)
			}
			beforeRecords := panel.records
			restarted.reconcileCurrentRuntimeNoop(context.Background(), panel, &currentRuntimeNoopAttempts{})
			if scenario == "unchanged" {
				if panel.records != beforeRecords+1 || !restarted.HasCurrentRuntimeNoop(context.Background(), panel.status) {
					t.Fatal("unchanged restart was not freshly verified")
				}
			} else if panel.records != beforeRecords || restarted.HasCurrentRuntimeNoop(context.Background(), panel.status) {
				t.Fatal("changed restart became success")
			}
			f.failSDK = false
			f.services[0].ImageID = originalID
		})
	}
}
