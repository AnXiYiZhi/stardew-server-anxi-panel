package docker

import "testing"

func TestResourceVolumeTargetsAreRestricted(t *testing.T) {
	targets, err := resourceVolumeTargets(`[{"Name":"a","Driver":"local","Mountpoint":"/var/lib/docker/volumes/a/_data"},{"Name":"remote","Driver":"nfs"}]`, []string{"a", "remote"})
	if err != nil || len(targets) != 1 || targets[0].Name != "a" {
		t.Fatalf("targets=%v err=%v", targets, err)
	}
	for _, raw := range []string{
		`truncated`,
		`[]`,
		`[{"Name":"unrelated","Driver":"local","Mountpoint":"/data"}]`,
		`[{"Name":"a","Driver":"local","Mountpoint":"/"}]`,
		`[{"Name":"a","Driver":"local","Mountpoint":"/data,readonly=false"}]`,
	} {
		if _, err := resourceVolumeTargets(raw, []string{"a"}); err == nil {
			t.Fatalf("accepted invalid metadata: %s", raw)
		}
	}
	targets, err = resourceVolumeTargets(`[{"Name":"a","Driver":"local","Mountpoint":"/data","Options":{"type":"none","o":"bind","device":"/external"}}]`, []string{"a"})
	if err != nil || len(targets) != 0 {
		t.Fatal("external bind volume must remain unknown")
	}
}
