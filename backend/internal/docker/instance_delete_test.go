package docker

import (
	"encoding/json"
	"testing"
)

func TestWorldDeletionReferenceAndIdentityGuards(t *testing.T) {
	var target deletionContainer
	if err := json.Unmarshal([]byte(`{"ID":"target-id","Config":{"Labels":{"com.docker.compose.project":"test-world","com.docker.compose.project.working_dir":"/data/instances/test-world","com.docker.compose.service":"server"}},"State":{"Status":"exited"},"Mounts":[{"Type":"volume","Name":"test-world_game-data"}]}`), &target); err != nil {
		t.Fatal(err)
	}
	plan := DeletionPlan{Project: "test-world", HostDir: "/data/instances/test-world", Containers: []string{target.ID}, Volumes: map[string]string{"test-world_game-data": "identity"}}
	client := &Client{}
	if !deletionContainerOwned(target, "test-world", "/host/panel/instances/test-world", "/data/instances/test-world") {
		t.Fatal("Panel container/host directory mapping rejected")
	}
	if err := client.validateDeletionReferences(plan, []deletionContainer{target}); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"running", "paused", "wrong-dir", "wrong-service", "foreign-holder", "new-container", "unknown-volume"} {
		t.Run(scenario, func(t *testing.T) {
			bytes, _ := json.Marshal(target)
			var copy deletionContainer
			_ = json.Unmarshal(bytes, &copy)
			all := []deletionContainer{copy}
			switch scenario {
			case "running":
				all[0].State.Running = true
			case "paused":
				all[0].State.Paused = true
			case "wrong-dir":
				all[0].Config.Labels["com.docker.compose.project.working_dir"] = "/data/instances/other"
			case "wrong-service":
				all[0].Config.Labels["com.docker.compose.service"] = "panel"
			case "foreign-holder":
				copy.ID = "foreign"
				copy.Config.Labels = map[string]string{}
				all = append(all, copy)
			case "new-container":
				copy.ID = "new"
				all = append(all, copy)
			case "unknown-volume":
				all[0].Mounts[0].Name = "shared-steam-download"
			}
			if err := client.validateDeletionReferences(plan, all); err == nil {
				t.Fatal("unsafe reference accepted")
			}
		})
	}
	v := deletionVolume{Name: "test-world_game-data", CreatedAt: "2026-09-05T01:00:00Z", Driver: "local"}
	before := deletionFingerprint(v)
	v.CreatedAt = "2026-09-05T02:00:00Z"
	if deletionFingerprint(v) == before {
		t.Fatal("recreated volume kept old identity")
	}
	if deletionWithin("/data/instances/world", "/data/instances/world-other") {
		t.Fatal("prefix collision")
	}
	if !deletionWithin(`C:\Users\test\instances\world`, "/run/desktop/mnt/host/c/Users/test/instances/world/.local-container/saves") {
		t.Fatal("Docker Desktop bind mapping rejected")
	}
	if deletionWithin(`C:\Users\test\instances\world`, "/run/desktop/mnt/host/c/Users/test/instances/world/../other") {
		t.Fatal("Desktop traversal accepted")
	}
	if deletionWithin(`C:\Users\test\instances\world`, "/run/desktop/mnt/host/d/Users/test/instances/world") {
		t.Fatal("wrong drive accepted")
	}
}
