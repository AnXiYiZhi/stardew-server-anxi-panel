package docker

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJunimo140SessionHolderMustBeStoppedKnownAuth(t *testing.T) {
	id := strings.Repeat("a", 64)
	for _, scenario := range []string{"stopped auth", "running", "restarting", "foreign", "server", "oneoff", "missing mount", "duplicate", "wrong id"} {
		t.Run(scenario, func(t *testing.T) {
			labels := map[string]string{"com.docker.compose.project": "world-a", "com.docker.compose.service": "steam-auth", "com.docker.compose.oneoff": "False"}
			state := map[string]bool{}
			record := map[string]any{"ID": id, "Config": map[string]any{"Labels": labels}, "State": state, "Mounts": []map[string]string{{"Type": "volume", "Name": "world-a-session"}}}
			switch scenario {
			case "running":
				state["Running"] = true
			case "restarting":
				state["Restarting"] = true
			case "foreign":
				labels["com.docker.compose.project"] = "world-b"
			case "server":
				labels["com.docker.compose.service"] = "server"
			case "oneoff":
				labels["com.docker.compose.oneoff"] = "True"
			case "missing mount":
				record["Mounts"] = nil
			case "wrong id":
				record["ID"] = strings.Repeat("b", 64)
			}
			records := []any{record}
			if scenario == "duplicate" {
				records = append(records, record)
			}
			raw, _ := json.Marshal(records)
			err := validateJunimo140SessionHolders(string(raw), []string{id}, "world-a", "world-a-session")
			if (err == nil) != (scenario == "stopped auth") {
				t.Fatalf("wrong session holder decision: %v", err)
			}
		})
	}
}
