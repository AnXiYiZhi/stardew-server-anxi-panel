package storage

import (
	"context"
	"testing"
)

func TestProvisionJournalFencesBothEndsAndGlobalJobs(t *testing.T) {
	s, closeStore := newStorageTestStore(t)
	defer closeStore()
	ctx := context.Background()
	for _, id := range []string{"source", "target", "other"} {
		if _, err := s.CreateInstance(ctx, CreateInstanceParams{ID: id, DriverID: DefaultDriverID, Name: id, DataDir: "/instances/" + id, State: InstanceStateAdminCreated, DriverPhase: "instance_provisioning"}); err != nil {
			t.Fatal(err)
		}
	}
	p := InstanceProvision{InstanceID: "target", TemplateID: "source", Token: "token"}
	job, err := s.CreateJob(ctx, CreateJobParams{Type: "install", TargetType: "instance", TargetID: "source"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.BeginInstanceProvision(ctx, p); err == nil {
		t.Fatal("active template accepted")
	}
	if _, err = s.db.Exec(`UPDATE jobs SET status='succeeded' WHERE id=?`, job.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.BeginInstanceProvision(ctx, p); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"source", "target"} {
		if _, err = s.CreateJob(ctx, CreateJobParams{Type: "writer", TargetType: "instance", TargetID: id}); err == nil {
			t.Fatal("late job accepted", id)
		}
	}
	if _, err = s.CreateJob(ctx, CreateJobParams{Type: "panel_update", TargetType: "panel", TargetID: "panel"}); err == nil {
		t.Fatal("global job accepted")
	}
	if _, err = s.CreateJob(ctx, CreateJobParams{Type: "reader", TargetType: "instance", TargetID: "other"}); err != nil {
		t.Fatal(err)
	}
	wrong := p
	wrong.Token = "foreign"
	if err = s.FinishInstanceProvision(ctx, wrong, false); err == nil {
		t.Fatal("foreign rollback accepted")
	}
	if err = s.FinishInstanceProvision(ctx, p, true); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateJob(ctx, CreateJobParams{Type: "writer", TargetType: "instance", TargetID: "source"}); err != nil {
		t.Fatal("source remained fenced", err)
	}
	target, err := s.GetInstance(ctx, "target")
	if err != nil || target.DriverPhase != "instance_ready" {
		t.Fatal(target, err)
	}
}
