package docker

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestJunimo140ServiceSnapshotRequiresCompleteBackedBindings(t *testing.T) {
	id, image := strings.Repeat("a", 64), "sha256:"+strings.Repeat("b", 64)
	for _, scenario := range []string{"managed", "stopped", "desktop alias", "foreign project", "oneoff", "privileged", "paused", "dead", "foreign image", "foreign bind", "wrong save", "readonly save", "missing save", "foreign game", "extra volume", "custom desktop", "duplicate mount", "duplicate service", "duplicate input", "different id"} {
		t.Run(scenario, func(t *testing.T) {
			root := "/fixtures/world-a"
			labels := map[string]string{"com.docker.compose.project": "world-a", "com.docker.compose.service": "server", "com.docker.compose.oneoff": "False"}
			mounts := []map[string]any{{"Type": "volume", "Name": "shared-game", "Destination": "/data/game", "RW": true}}
			for target, leaf := range map[string]string{"/config/xdg/config/StardewValley": "saves", "/data/Mods": "mods", "/data/settings": "settings", "/data/control": "control"} {
				mounts = append(mounts, map[string]any{"Type": "bind", "Source": root + "/.local-container/" + leaf, "Destination": target, "RW": true})
			}
			mounts = append(mounts, map[string]any{"Type": "volume", "Name": strings.Repeat("c", 64), "Destination": "/config", "RW": true})
			state := map[string]any{"Running": true, "Status": "running"}
			host := map[string]any{}
			record := map[string]any{"ID": id, "Image": image, "Config": map[string]any{"Image": "fixed@" + image, "Labels": labels}, "State": state, "HostConfig": host, "Mounts": mounts}
			switch scenario {
			case "stopped":
				state["Running"], state["Status"] = false, "exited"
			case "desktop alias":
				root = "E:/fixtures/world-a"
				for _, mount := range mounts {
					if mount["Type"] == "bind" {
						mount["Source"] = strings.Replace(mount["Source"].(string), "/fixtures/world-a", "/run/desktop/mnt/host/e/fixtures/world-a", 1)
					}
				}
			case "foreign project":
				labels["com.docker.compose.project"] = "foreign"
			case "oneoff":
				labels["com.docker.compose.oneoff"] = "True"
			case "privileged":
				host["Privileged"] = true
			case "paused":
				state["Paused"] = true
			case "dead":
				state["Dead"] = true
			case "foreign image":
				record["Image"] = "mutable:latest"
			case "foreign bind":
				mounts[1]["Source"] = "/elsewhere/saves"
			case "wrong save", "readonly save", "missing save":
				for i, mount := range mounts {
					if mount["Destination"] == "/config/xdg/config/StardewValley" {
						if scenario == "wrong save" {
							mount["Source"] = root + "/.local-container/other"
						} else if scenario == "readonly save" {
							mount["RW"] = false
						} else {
							mounts = append(mounts[:i], mounts[i+1:]...)
							record["Mounts"] = mounts
						}
						break
					}
				}
			case "foreign game":
				mounts[0]["Name"] = "foreign-game"
			case "extra volume":
				mounts = append(mounts, map[string]any{"Type": "volume", "Name": "unprotected", "Destination": "/other"})
				record["Mounts"] = mounts
			case "custom desktop":
				mounts[len(mounts)-1]["Name"] = "custom-config"
			case "duplicate mount":
				mounts = append(mounts, mounts[0])
				record["Mounts"] = mounts
			case "different id":
				record["ID"] = strings.Repeat("d", 64)
			}
			records, ids := []any{record}, []string{id}
			if scenario == "duplicate service" {
				other := map[string]any{}
				for k, v := range record {
					other[k] = v
				}
				other["ID"] = strings.Repeat("d", 64)
				records, ids = append(records, other), append(ids, other["ID"].(string))
			}
			if scenario == "duplicate input" {
				records, ids = append(records, record), append(ids, id)
			}
			raw, _ := json.Marshal(records)
			got, err := parseJunimo140ServiceSnapshots(string(raw), ids, RuntimeJunimo140WorldScope{Project: "world-a", HostDataDir: root}, map[string]bool{"shared-game": true})
			allowed := scenario == "managed" || scenario == "stopped" || scenario == "desktop alias"
			if (err == nil) != allowed {
				t.Fatalf("wrong service ownership decision: %v", err)
			}
			if allowed && (len(got) != 1 || got[0].Running != (scenario != "stopped") || got[0].GameVolume != "shared-game") {
				t.Fatal("wrong bounded service projection")
			}
		})
	}
}

func TestJunimo140ServiceStopValidatesEntireSetBeforeDocker(t *testing.T) {
	image := "sha256:" + strings.Repeat("b", 64)
	snapshots := []RuntimeJunimo140ServiceSnapshot{
		{Project: "world-a", Service: "server", RuntimeServiceMetadata: RuntimeServiceMetadata{ContainerID: strings.Repeat("a", 64), ImageID: image}, Running: true},
		{Project: "world-b", Service: "server", RuntimeServiceMetadata: RuntimeServiceMetadata{ContainerID: strings.Repeat("c", 64), ImageID: image}, Running: true},
	}
	client := NewClient(Options{DockerPath: "must-not-execute-shared-stop"})
	err := client.RuntimeStopJunimo140Services(context.Background(), t.TempDir(), snapshots, map[string][]string{"world-a/server": {image}, "world-b/server": {"sha256:" + strings.Repeat("d", 64)}})
	if err == nil || !strings.Contains(err.Error(), "no services stopped") {
		t.Fatalf("partial stop attempted before complete validation: %v", err)
	}
}
