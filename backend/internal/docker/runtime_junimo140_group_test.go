package docker

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJunimo140SharedGameHoldersRejectUnknownAndLiveWriters(t *testing.T) {
	for _, scenario := range []string{"stopped known", "live discovery", "live after stop", "restarting after stop", "foreign project", "one shot", "unknown service", "wrong mount", "missing mount", "duplicate inspect", "changed inspect id"} {
		t.Run(scenario, func(t *testing.T) {
			id := strings.Repeat("a", 64)
			labels := map[string]string{"com.docker.compose.project": "world-a", "com.docker.compose.service": "server", "com.docker.compose.oneoff": "False"}
			mounts := []map[string]string{{"Type": "volume", "Name": "shared-game", "Destination": "/data/game"}}
			state := map[string]bool{"Running": false, "Restarting": false}
			quiescent, accepted := true, false
			switch scenario {
			case "stopped known":
				accepted = true
			case "live discovery":
				state["Running"], quiescent, accepted = true, false, true
			case "live after stop":
				state["Running"] = true
			case "restarting after stop":
				state["Restarting"] = true
			case "foreign project":
				labels["com.docker.compose.project"] = "unrelated"
			case "one shot":
				labels["com.docker.compose.oneoff"] = "True"
			case "unknown service":
				labels["com.docker.compose.service"] = "attach-cli"
			case "wrong mount":
				mounts[0]["Destination"] = "/game"
			case "missing mount":
				mounts = nil
			}
			holder := map[string]any{"ID": id, "Config": map[string]any{"Labels": labels, "Env": []string{"PRIVATE_VALUE=not-returned"}}, "State": state, "Mounts": mounts}
			holders := []map[string]any{holder}
			if scenario == "duplicate inspect" {
				holders = append(holders, holder)
			}
			if scenario == "changed inspect id" {
				holder["ID"] = strings.Repeat("b", 64)
			}
			raw, _ := json.Marshal(holders)
			err := validateJunimo140GameHolders(string(raw), []string{id}, map[string]bool{"world-a": true}, "shared-game", quiescent)
			if (err == nil) != accepted {
				t.Fatalf("wrong holder decision: %v", err)
			}
			if err != nil && strings.Contains(err.Error(), "PRIVATE_VALUE") {
				t.Fatal("inspect environment leaked")
			}
		})
	}
}
