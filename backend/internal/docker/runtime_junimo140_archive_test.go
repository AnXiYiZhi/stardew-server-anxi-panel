package docker

import (
	"strings"
	"testing"
)

func TestRuntimeInstanceArchiveScope(t *testing.T) {
	request := RuntimeInstanceArchiveRequest{Project: "world-a", InstanceHostDir: "/srv/panel/world-a", RecoveryHostDir: "/srv/panel/world-a/.local-container/junimo140-group/apply_" + strings.Repeat("a", 24), Filename: strings.Repeat("b", 12) + ".tar", ImageRef: "sdvd/server:1.5.0-preview.140", ImageID: "sha256:" + strings.Repeat("c", 64)}
	if err := validateRuntimeInstanceArchive(request); err != nil {
		t.Fatal(err)
	}
	windows := request
	windows.InstanceHostDir = `E:\panel\world-a`
	windows.RecoveryHostDir = `E:\panel\world-a\.local-container\junimo140-group\apply_` + strings.Repeat("a", 24)
	if err := validateRuntimeInstanceArchive(windows); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*RuntimeInstanceArchiveRequest)
	}{
		{"root", func(r *RuntimeInstanceArchiveRequest) { r.InstanceHostDir = "/" }},
		{"relative", func(r *RuntimeInstanceArchiveRequest) { r.InstanceHostDir = "srv/panel/world-a" }},
		{"wrong project", func(r *RuntimeInstanceArchiveRequest) { r.Project = "world-b" }},
		{"parent traversal", func(r *RuntimeInstanceArchiveRequest) { r.InstanceHostDir = "/srv/panel/../world-a" }},
		{"mount injection", func(r *RuntimeInstanceArchiveRequest) { r.InstanceHostDir = "/srv,type=volume/world-a" }},
		{"unscoped recovery", func(r *RuntimeInstanceArchiveRequest) { r.RecoveryHostDir = "/srv/backups" }},
		{"archive traversal", func(r *RuntimeInstanceArchiveRequest) { r.Filename = "../original.tar" }},
		{"mutable image", func(r *RuntimeInstanceArchiveRequest) { r.ImageID = "sdvd/server:latest" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := request
			tc.change(&bad)
			if validateRuntimeInstanceArchive(bad) == nil {
				t.Fatal("unsafe archive request accepted")
			}
		})
	}
}
