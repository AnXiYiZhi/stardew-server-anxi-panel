package docker

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// This executable fixture exercises the production subprocess/Raw return ABI.
// It never invokes Docker or mounts a volume, and never substitutes a Runner.
type junimoRawRunnerFixture struct {
	client *Client
	dir    string
}

type junimoRawRunnerState struct {
	Reader    string
	Malformed bool
	Fail      bool
	Active    bool
	Name      string
	ID        string
	Volume    string
	Args      []string
	Creates   int
	Starts    int
	Removes   int
	Runs      int
	StdinRead bool
}

const rawRunnerImage = "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
const rawRunnerImageRef = "sdvd/server:1.5.0-preview.140"

func newJunimoRawRunnerFixture(t *testing.T, reader string, malformed, fail bool) junimoRawRunnerFixture {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("executable subprocess fixture requires Linux")
	}
	f := junimoRawRunnerFixture{dir: t.TempDir()}
	state, err := json.Marshal(junimoRawRunnerState{Reader: reader, Malformed: malformed, Fail: fail})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.dir, "state.json"), state, 0600); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
	script := "#!/bin/sh\nexec " + quote(self) + " -test.run='^TestJunimo140RawRunnerFixtureProcess$' -- --junimo-raw-fixture " + quote(f.dir) + " \"$@\"\n"
	binary := filepath.Join(f.dir, "docker-fixture")
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	f.client = NewClient(Options{DockerPath: binary})
	return f
}

func (f junimoRawRunnerFixture) state(t *testing.T) junimoRawRunnerState {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state junimoRawRunnerState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	return state
}

// The shell executable above starts only this function in a child test process.
// os.Exit avoids the testing package's PASS text entering structured stdout.
func TestJunimo140RawRunnerFixtureProcess(t *testing.T) {
	marker := -1
	for i, arg := range os.Args {
		if arg == "--junimo-raw-fixture" {
			marker = i
			break
		}
	}
	if marker < 0 {
		return
	}
	if marker+2 >= len(os.Args) {
		os.Exit(81)
	}
	dir, args := os.Args[marker+1], os.Args[marker+2:]
	path := filepath.Join(dir, "state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		os.Exit(82)
	}
	var state junimoRawRunnerState
	if json.Unmarshal(data, &state) != nil {
		os.Exit(83)
	}
	save := func() {
		data, err := json.Marshal(state)
		if err != nil || os.WriteFile(path, data, 0600) != nil {
			os.Exit(84)
		}
	}
	outputJSON := func(value any) {
		data, err := json.Marshal(value)
		if err != nil {
			os.Exit(85)
		}
		fmt.Println(string(data))
	}
	valueAfter := func(values []string, flag string) string {
		for i := 0; i+1 < len(values); i++ {
			if values[i] == flag {
				return values[i+1]
			}
		}
		return ""
	}
	payload := func(values []string) {
		if state.Malformed {
			fmt.Println("invalid-metadata-frame")
			return
		}
		switch state.Reader {
		case "sdk":
			if len(values) < 4 || !strings.Contains(strings.Join(values, " "), inspectJunimo140SDKScript) {
				os.Exit(86)
			}
			files := [2][]byte{sdkInspectionELFFixture(512), sdkInspectionELFFixture(640)}
			for i, file := range files {
				offset, e1 := strconv.ParseUint(values[len(values)-4+i*2], 10, 64)
				length, e2 := strconv.ParseUint(values[len(values)-3+i*2], 10, 64)
				if e1 != nil || e2 != nil || !sdkELFRangeWithin(uint64(len(file)), offset, length) {
					os.Exit(87)
				}
				fmt.Printf("%d %s %s\n", len(file), base64.StdEncoding.EncodeToString(file[:64]), base64.StdEncoding.EncodeToString(file[offset:offset+length]))
			}
		case "smapi":
			fmt.Printf("PRESENT=1\nFILES=1\nVERSION=4.1.10+abcdef0\nBUILD_ID=fixture-build\nIMAGE_BUILD_ID=fixture-build\nHASH:StardewModdingAPI.dll=%s\n", strings.Repeat("a", 64))
		case "assets":
			fmt.Printf("%s  JunimoServer.dll\n%s  manifest.json\n", strings.Repeat("a", 64), strings.Repeat("b", 64))
		case "game":
			input, err := io.ReadAll(os.Stdin)
			if err != nil || !strings.Contains(string(input), "Game.dll") {
				os.Exit(88)
			}
			state.StdinRead = true
			fmt.Println("1 7 1 3")
		default:
			fmt.Println("opaque-fixture-output")
		}
		if state.Fail {
			fmt.Fprintln(os.Stderr, "fixture helper failed")
			save()
			os.Exit(17)
		}
	}
	switch args[0] {
	case "raw-output":
		payload(args)
	case "ps": // unused staging has no holders
	case "volume":
		volume := args[len(args)-1]
		outputJSON([]any{map[string]any{"Name": volume, "Labels": map[string]string{"com.anxi-panel.smapi-update-staging": "true", "com.anxi-panel.compose-project": "world-a"}}})
	case "image":
		if strings.Contains(strings.Join(args, " "), "Config.Volumes") {
			fmt.Printf("%q|{}\n", rawRunnerImage)
		} else {
			fmt.Printf("%q|[\"sdvd/server@sha256:f23fdc3a3e999032be363fa43291c2d72626555913ef2b3dcf71b0aec53c49db\"]\n", rawRunnerImage)
		}
	case "run":
		state.Runs++
		payload(args)
	case "create":
		state.Name = valueAfter(args, "--name")
		digest := sha256.Sum256([]byte(state.Name))
		state.ID = hex.EncodeToString(digest[:])
		state.Volume = ""
		for _, arg := range args {
			if strings.HasPrefix(arg, "com.anxi-panel.game-data-volume=") {
				state.Volume = strings.TrimPrefix(arg, "com.anxi-panel.game-data-volume=")
			}
		}
		state.Active, state.Args = true, args
		state.Creates++
		fmt.Println(state.ID)
	case "start":
		state.Starts++
		payload(state.Args)
	case "container":
		switch args[1] {
		case "ls":
			filter := valueAfter(args, "--filter")
			if state.Active && (filter == "name=^/"+state.Name+"$" || filter == "id="+state.ID) {
				fmt.Println(state.ID)
			}
		case "inspect":
			if !state.Active || args[len(args)-1] != state.ID {
				os.Exit(89)
			}
			declarations, mounts := []any{}, []any{}
			if state.Volume != "" {
				declarations = append(declarations, map[string]any{"Type": "volume", "Source": state.Volume, "Target": "/game", "ReadOnly": true, "VolumeOptions": map[string]bool{"NoCopy": true}})
				mounts = append(mounts, map[string]any{"Type": "volume", "Name": state.Volume, "Destination": "/game", "RW": false})
			}
			outputJSON([]any{map[string]any{"ID": state.ID, "Name": "/" + state.Name, "Image": rawRunnerImage,
				"Config":     map[string]any{"Image": rawRunnerImage, "User": "0:0", "Labels": map[string]string{"com.anxi-panel.current-runtime-read": "true", "com.anxi-panel.game-data-volume": state.Volume}, "Volumes": map[string]any{}},
				"HostConfig": map[string]any{"NetworkMode": "none", "ReadonlyRootfs": true, "Privileged": false, "AutoRemove": false, "CapDrop": []string{"ALL"}, "CapAdd": []string{"DAC_READ_SEARCH"}, "SecurityOpt": []string{"no-new-privileges"}, "Binds": []string{}, "VolumesFrom": []string{}, "Mounts": declarations}, "Mounts": mounts}})
		case "rm":
			if !state.Active || args[len(args)-1] != state.ID {
				os.Exit(90)
			}
			state.Active = false
			state.Removes++
			fmt.Println(state.ID)
		default:
			os.Exit(91)
		}
	default:
		os.Exit(92)
	}
	save()
	os.Exit(0)
}

func TestJunimo140RawRunnerKeepsStructuredStdoutPrivate(t *testing.T) {
	f := newJunimoRawRunnerFixture(t, "raw", false, false)
	result, raw, err := f.client.runWithEnvironmentRaw(context.Background(), "raw ABI regression", f.dir, time.Second*10, nil, "raw-output")
	if err != nil || result.Stdout != "" || raw != "opaque-fixture-output\n" || result.StdoutTruncated {
		t.Fatal("production raw subprocess ABI changed or stdout escaped its private return")
	}
}

func TestJunimo140RawRunnerReaders(t *testing.T) {
	for _, current := range []bool{false, true} {
		name := "staging"
		if current {
			name = "current"
		}
		for _, reader := range []string{"sdk", "smapi", "game", "evidence"} {
			t.Run(name+"/"+reader, func(t *testing.T) {
				f := newJunimoRawRunnerFixture(t, reader, false, false)
				volume := "world-a_anxi-smapi-update-" + strings.Repeat("c", 24)
				var projects []string
				if current {
					volume, projects = "existing-game", []string{"world-a"}
				}
				ctx := context.Background()
				switch reader {
				case "sdk":
					var stats RuntimeGameFileStats
					var err error
					if current {
						stats, err = f.client.RuntimeInspectCurrentJunimo140SDK(ctx, f.dir, volume, rawRunnerImageRef, rawRunnerImage, projects)
					} else {
						stats, err = f.client.RuntimeInspectJunimo140SDK(ctx, f.dir, volume, rawRunnerImageRef, rawRunnerImage)
					}
					if err != nil || stats.SDKFiles != 2 || stats.SDKBytes != 1152 {
						t.Fatalf("SDK structured raw result did not reach production parser: %v", err)
					}
				case "smapi":
					metadata, err := f.client.readSMAPIMetadataFromImageIDWithCurrentScope(ctx, f.dir, volume, rawRunnerImage, projects)
					if err != nil || !metadata.Present || !metadata.RequiredFiles || metadata.Version != "4.1.10+abcdef0" || metadata.CoreHashes["StardewModdingAPI.dll"] != strings.Repeat("a", 64) {
						t.Fatalf("SMAPI structured raw result was lost: %v", err)
					}
				case "game":
					stats, err := f.client.verifyJunimo140GameFiles(ctx, f.dir, volume, rawRunnerImageRef, rawRunnerImage, "sha256", map[string]string{"Game.dll": strings.Repeat("a", 64), ".steam-sdk/linux64/steamclient.so": strings.Repeat("b", 64)}, projects)
					if err != nil || stats != (RuntimeGameFileStats{1, 7, 1, 3}) || !f.state(t).StdinRead {
						t.Fatalf("stdin statistics raw result was lost: %v", err)
					}
				case "evidence":
					data, err := f.client.readJunimo140GameEvidence(ctx, f.dir, volume, rawRunnerImageRef, rawRunnerImage, "sdk", projects)
					if err != nil || string(data) != "opaque-fixture-output\n" {
						t.Fatalf("game evidence raw result was lost: %v", err)
					}
				}
				state := f.state(t)
				expected := 1
				if reader == "sdk" {
					expected = 3
				}
				if state.Active || (current && (state.Creates != expected || state.Starts != expected || state.Removes != expected || state.Runs != 0)) || (!current && (state.Runs != expected || state.Creates != 0)) {
					t.Fatal("reader did not follow its actual staging/current subprocess lifecycle")
				}
			})
		}
	}
	t.Run("image-assets", func(t *testing.T) {
		f := newJunimoRawRunnerFixture(t, "assets", false, false)
		hashes, err := f.client.RuntimeReadJunimo140ModHashes(context.Background(), f.dir, rawRunnerImageRef, rawRunnerImage)
		if err != nil || len(hashes) != 2 || hashes["JunimoServer.dll"] != strings.Repeat("a", 64) || hashes["manifest.json"] != strings.Repeat("b", 64) {
			t.Fatalf("image-only raw identity was lost: %v", err)
		}
		state := f.state(t)
		if state.Active || state.Creates != 1 || state.Starts != 1 || state.Removes != 1 {
			t.Fatal("image-only reader helper was not precisely removed")
		}
	})
}

func TestJunimo140RawRunnerSDKFailureDoesNotExposeMetadata(t *testing.T) {
	for _, current := range []bool{false, true} {
		for _, commandFailure := range []bool{false, true} {
			t.Run(fmt.Sprintf("current=%t/commandFailure=%t", current, commandFailure), func(t *testing.T) {
				f := newJunimoRawRunnerFixture(t, "sdk", !commandFailure, commandFailure)
				volume := "world-a_anxi-smapi-update-" + strings.Repeat("c", 24)
				var projects []string
				if current {
					volume, projects = "existing-game", []string{"world-a"}
				}
				_, err := f.client.readJunimo140SDKMetadataWithScope(context.Background(), f.dir, volume, rawRunnerImageRef, rawRunnerImage, [2]sdkELFRange{{0, 64}, {0, 64}}, nil, projects)
				encoded := base64.StdEncoding.EncodeToString(sdkInspectionELFFixture(512)[:64])
				if err == nil || strings.Contains(err.Error(), encoded) || strings.Contains(err.Error(), "invalid-metadata-frame") {
					t.Fatal("invalid SDK metadata was accepted or exposed through the public error")
				}
				if state := f.state(t); state.Active || (current && (state.Creates != 1 || state.Removes != 1)) {
					t.Fatal("failed SDK read left its owned helper behind")
				}
			})
		}
	}
}
