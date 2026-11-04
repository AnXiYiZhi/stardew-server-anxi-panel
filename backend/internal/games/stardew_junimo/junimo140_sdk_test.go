package stardew_junimo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func installedSDKFixture(build, manifest string) []byte {
	return []byte(fmt.Sprintf(`"AppState" { "appid" "1007" "buildid" "%s" "StateFlags" "4" "installdir" "Steamworks Shared" "InstalledDepots" { "1006" { "manifest" "%s" } } }`, build, manifest))
}

func completeJunimo140GameEvidence(t *testing.T, fake *fakeDocker, sdk []byte) {
	t.Helper()
	read, err := fake.RuntimeReadContentVersions(context.Background(), "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	// This catalog is the unmodified upstream list of Content file names and
	// hashes, not a game payload. Its SHA is independently pinned in production.
	content, err := os.ReadFile(filepath.Join("testdata", "junimo140-content-hashes.json"))
	if err != nil {
		t.Fatal(err)
	}
	fake.gameIntegrityEvidence = map[string][]byte{"game": read.GameManifest, "sdk": sdk, "content": content}
}

func TestJunimo140SDKIdentityUsesInstalledDepot(t *testing.T) {
	for _, pair := range [][2]string{{"20939719", "6403079453713498174"}, {"99999999", "7900000000000000001"}, {"222", "123"}} {
		t.Run(pair[0], func(t *testing.T) {
			data := installedSDKFixture(pair[0], pair[1])
			parsed, err := parseCompleteSDKManifest(data)
			if err != nil || parsed.BuildID != pair[0] || parsed.InstalledDepots["1006"] != pair[1] {
				t.Fatalf("independently installed SDK rejected: %#v %v", parsed, err)
			}
			marker, err := installedSDKDownloadMarker(data, paneldocker.RuntimeGameFileStats{SDKFiles: 2, SDKBytes: 123456}, time.Date(2026, 11, 4, 18, 0, 0, 0, time.FixedZone("CST", 8*60*60)))
			if err != nil || fmt.Sprint(marker.ManifestID) != pair[1] || marker.AppID != 1007 || marker.DepotID != 1006 || marker.TargetOS != "linux" || marker.TotalFiles != 2 || marker.TotalBytes != 123456 || marker.DownloadedAt != "2026-11-04T10:00:00Z" {
				t.Fatalf("marker does not describe actual installed SDK: %#v %v", marker, err)
			}
		})
	}
}

func TestJunimo140SDKIdentityRejectsIncompleteOrAmbiguousInstallation(t *testing.T) {
	valid := string(installedSDKFixture("99999999", "7900000000000000001"))
	cases := map[string]string{
		"wrong app":       strings.Replace(valid, `"appid" "1007"`, `"appid" "413150"`, 1),
		"incomplete":      strings.Replace(valid, `"StateFlags" "4"`, `"StateFlags" "6"`, 1),
		"wrong depot":     strings.Replace(valid, `"1006" {`, `"1005" {`, 1),
		"zero manifest":   strings.Replace(valid, "7900000000000000001", "0", 1),
		"overflow":        strings.Replace(valid, "7900000000000000001", "18446744073709551616", 1),
		"duplicate depot": strings.Replace(valid, `"1006" {`, `"1006" { "manifest" "123" } "1006" {`, 1),
		"extra depot":     strings.Replace(valid, `"1006" {`, `"1005" { "manifest" "123" } "1006" {`, 1),
		"missing depot":   string(manifestFixture("1007", "99999999", "4", "Steamworks Shared")),
		"invalid":         "not an ACF",
	}
	cases["missing depot"] = strings.Replace(cases["missing depot"], `"InstalledDepots"`, `"UnknownDepots"`, 1)
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseCompleteSDKManifest([]byte(data)); err == nil {
				t.Fatal("invalid SDK installation accepted")
			}
			if _, err := installedSDKDownloadMarker([]byte(data), paneldocker.RuntimeGameFileStats{SDKFiles: 2, SDKBytes: 123456}, time.Now()); err == nil {
				t.Fatal("invalid SDK installation published a marker")
			}
		})
	}
	for _, stats := range []paneldocker.RuntimeGameFileStats{{}, {SDKFiles: 1, SDKBytes: 100}, {SDKFiles: 2}, {SDKFiles: 2, SDKBytes: 100, GameFiles: 1}, {SDKFiles: 2, SDKBytes: 100, GameBytes: 1}} {
		if _, err := installedSDKDownloadMarker([]byte(valid), stats, time.Now()); err == nil {
			t.Fatalf("unverified SDK statistics accepted: %#v", stats)
		}
	}
}

type sdkPreflightDocker struct {
	*runtimeUpdateFakeDocker
	content paneldocker.RuntimeContentRead
	err     error
}

func (f *sdkPreflightDocker) RuntimeReadContentVersions(_ context.Context, _, _, _ string) (paneldocker.RuntimeContentRead, error) {
	f.record("read game SDK manifests readonly")
	return f.content, f.err
}

func TestJunimo140SDKPreflightChecksInstallationBeforeStopping(t *testing.T) {
	validGame := []byte(`"AppState" { "appid" "413150" "buildid" "16826371" "StateFlags" "4" "installdir" "Stardew Valley" "InstalledDepots" { "413153" { "manifest" "6005910083361727734" } } }`)
	for _, test := range []struct {
		name string
		game []byte
		sdk  []byte
		err  error
		ok   bool
	}{
		{name: "current public SDK", game: validGame, sdk: installedSDKFixture("99999999", "7900000000000000001"), ok: true},
		{name: "game build mismatch", game: []byte(strings.Replace(string(validGame), "16826371", "16826372", 1)), sdk: installedSDKFixture("99999999", "7900000000000000001")},
		{name: "SDK missing", game: validGame},
		{name: "SDK pending install", game: validGame, sdk: []byte(strings.Replace(string(installedSDKFixture("99999999", "123")), `"StateFlags" "4"`, `"StateFlags" "6"`, 1))},
		{name: "source read failed", err: errors.New("read unavailable")},
	} {
		t.Run(test.name, func(t *testing.T) {
			driver, _, instance, fake := setupRuntimeUpdateDriver(t, storage.InstanceStateRunning)
			reader := &sdkPreflightDocker{runtimeUpdateFakeDocker: fake, content: paneldocker.RuntimeContentRead{GameManifest: test.game, SDKManifest: test.sdk}, err: test.err}
			if err := driver.preflightJunimo140Content(context.Background(), reader, instance, "sdvd/server:1.5.0-preview.125"); (err == nil) != test.ok {
				t.Fatalf("preflight success=%v want=%v: %v", err == nil, test.ok, err)
			}
			if len(fake.calls) != 1 || fake.calls[0] != "read game SDK manifests readonly" {
				t.Fatalf("manifest inspection made mutating calls: %v", fake.calls)
			}
		})
	}
}

func TestJunimo140GameCatalogRetainsOnlyFixedGameFiles(t *testing.T) {
	pins, err := fixedJunimo140GamePins()
	if err != nil {
		t.Fatal(err)
	}
	if len(pins.CoreFiles) != 226 || pins.Game.BuildID != "16826371" || pins.Content.Entries != 3560 {
		t.Fatalf("fixed game integrity weakened: %#v", pins.Game)
	}
	for name := range pins.CoreFiles {
		if strings.HasPrefix(name, ".steam-sdk/") {
			t.Fatal("independently installed SDK is part of fixed game hashes")
		}
	}
}

func TestJunimo140SDKDryRunRejectsIncompleteSourceBeforeTargetPullOrStop(t *testing.T) {
	driver, store, instance, fake := setupRuntimeUpdateDriver(t, storage.InstanceStateRunning)
	driver.runtimeManifestProvider = sjconfig.BuiltInRuntimeStackManifest
	read, err := fake.fakeDocker.RuntimeReadContentVersions(context.Background(), "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	reader := &sdkPreflightDocker{runtimeUpdateFakeDocker: fake, content: paneldocker.RuntimeContentRead{GameManifest: read.GameManifest}}
	driver.docker = reader
	inspection := driver.EffectiveRuntimeInspection(instance)
	status := RuntimeUpdateDryRunStatus{DryRunID: newRuntimeDryRunID(), Phase: RuntimeUpdatePhaseStarting, Current: inspection.Current, Target: inspection.Recommended, Checks: []RuntimeUpdateDryRunCheck{}, Logs: []RuntimeUpdateDryRunLog{}, Warnings: []string{}}
	done := make(chan error, 1)
	job, err := driver.jobs.Start(context.Background(), jobs.Spec{Type: RuntimeUpdateDryRunJobType, TargetType: "instance", TargetID: instance.ID, Run: func(ctx context.Context, job *jobs.Context) error {
		err := driver.runRuntimeUpdateDryRun(ctx, job, reader, instance, status)
		done <- err
		return err
	}})
	if err != nil || job.ID == "" {
		t.Fatalf("start dry-run fixture: %v", err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("dry-run accepted missing SDK")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("dry-run did not finish")
	}
	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusFailed)
	actual, err := driver.RuntimeUpdateDryRunStatus(instance)
	if err != nil || actual.Phase != RuntimeUpdatePhaseFailed || actual.ErrorCode != "game_sdk_installation_invalid" {
		t.Fatalf("dry-run did not expose source failure: %#v %v", actual, err)
	}
	for _, call := range fake.calls {
		for _, forbidden := range []string{"pull", " up", " stop", " down", " rm", "restart", "archive"} {
			if strings.Contains(call, forbidden) {
				t.Fatalf("preflight failure reached later or mutating work: %s", call)
			}
		}
	}
}
