package docker

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJunimo140HelperRecoveryRejectsUnknownBeforeStopping(t *testing.T) {
	id, image := strings.Repeat("a", 64), "sha256:"+strings.Repeat("b", 64)
	scope := RuntimeJunimo140HelperScope{ApplyID: "apply_" + strings.Repeat("c", 24), Projects: []string{"world-a"}, ImageID: image}
	for _, scenario := range []string{"owned", "wrong project", "wrong image", "wrong ref", "wrong name", "not auto removed", "unknown operation", "wrong owner", "duplicate", "different id"} {
		t.Run(scenario, func(t *testing.T) {
			labels := map[string]string{"com.anxi-panel.junimo140-helper": "true", "com.anxi-panel.compose-project": "world-a", "com.anxi-panel.runtime-update-apply-id": scope.ApplyID, "com.anxi-panel.junimo140-operation": "smapi-clone"}
			record := map[string]any{"ID": id, "Image": image, "Name": "/anxi-j140-" + strings.Repeat("c", 24) + "-smapi-clone", "Config": map[string]any{"Image": image, "Labels": labels}, "HostConfig": map[string]bool{"AutoRemove": true}}
			switch scenario {
			case "wrong project":
				labels["com.anxi-panel.compose-project"] = "foreign"
			case "wrong image":
				record["Image"] = "sha256:" + strings.Repeat("d", 64)
			case "wrong ref":
				record["Config"].(map[string]any)["Image"] = "mutable:latest"
			case "wrong name":
				record["Name"] = "/ordinary-game"
			case "not auto removed":
				record["HostConfig"] = map[string]bool{"AutoRemove": false}
			case "unknown operation":
				labels["com.anxi-panel.junimo140-operation"] = "unrelated"
			case "wrong owner":
				labels["com.anxi-panel.runtime-update-apply-id"] = "apply_" + strings.Repeat("d", 24)
			case "different id":
				record["ID"] = strings.Repeat("d", 64)
			}
			records := []any{record}
			if scenario == "duplicate" {
				records = append(records, record)
			}
			raw, _ := json.Marshal(records)
			err := validateJunimo140Helpers(string(raw), []string{id}, scope)
			if (err == nil) != (scenario == "owned") {
				t.Fatalf("wrong helper recovery decision: %v", err)
			}
		})
	}
}
