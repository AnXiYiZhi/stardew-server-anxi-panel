package storage

import (
	"context"
	"errors"
	"testing"
)

func TestInstanceDeletionTransactionAndTombstone(t *testing.T) {
	s, closeStore := newStorageTestStore(t)
	defer closeStore()
	ctx := context.Background()
	for _, id := range []string{"configured-default", "target", "other"} {
		if _, err := s.CreateInstance(ctx, CreateInstanceParams{ID: id, DriverID: DefaultDriverID, Name: "same name", DataDir: "/instances/" + id, State: InstanceStateStopped}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.BeginInstanceDeletion(ctx, "configured-default", "configured-default", "{}"); !errors.Is(err, ErrConflict) {
		t.Fatal("default protection", err)
	}
	job, err := s.CreateJob(ctx, CreateJobParams{Type: "backup", TargetType: "instance", TargetID: "target"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.BeginInstanceDeletion(ctx, "target", "configured-default", "{}"); !errors.Is(err, ErrConflict) {
		t.Fatal("active job", err)
	}
	if _, err = s.db.Exec(`UPDATE jobs SET status='succeeded' WHERE id=?`, job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(`INSERT INTO job_logs(job_id,level,message,sequence) VALUES (?,'info','test',1)`, job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(`INSERT INTO audit_logs(action,target_type,target_id) VALUES ('backup','instance','target')`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateJob(ctx, CreateJobParams{Type: "backup", TargetType: "instance", TargetID: "other"}); err != nil {
		t.Fatal(err)
	}
	if err = s.BeginInstanceDeletion(ctx, "target", "configured-default", `{"immutable":true}`); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`INSERT INTO save_identities(instance_id,stable_save_id,base_save_id,first_seen_at,last_seen_at) VALUES ('target','save','save','now','now')`,
		`INSERT INTO player_roster(instance_id,stable_save_id,player_id,display_name,first_seen_at,last_seen_at,snapshot_observed_at) VALUES ('target','save','player','synthetic','now','now','now')`,
		`INSERT INTO player_events(id,instance_id,stable_save_id,event_type,player_id,player_name,occurred_at) VALUES ('event','target','save','seen','player','synthetic','now')`,
		`INSERT INTO control_commands(command_id,instance_id,command_type,status,submitted_at,updated_at) VALUES ('command','target','save','succeeded','now','now')`,
		`INSERT INTO restart_schedules(instance_id,shutdown_time,startup_time,timezone) VALUES ('target','01:00','02:00','UTC')`,
	} {
		if _, err = s.db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.CreateJob(ctx, CreateJobParams{Type: "start", TargetType: "instance", TargetID: "target"}); err == nil {
		t.Fatal("late job accepted")
	}
	if _, err = s.CreateJob(ctx, CreateJobParams{Type: "panel_update", TargetType: "system", TargetID: "panel"}); err == nil {
		t.Fatal("global maintenance admitted during deletion")
	}
	if _, err = s.RenameInstance(ctx, "target", "renamed"); err == nil {
		t.Fatal("late mutation accepted")
	}
	record, err := s.GetInstanceDeletion(ctx, "target")
	if err != nil || record.Completed || record.Plan != `{"immutable":true}` {
		t.Fatal(record, err)
	}
	if err = s.CompleteInstanceDeletion(ctx, "target"); err != nil {
		t.Fatal(err)
	}
	if err = s.CompleteInstanceDeletion(ctx, "target"); err != nil {
		t.Fatal("repeat completion", err)
	}
	for _, query := range []string{`SELECT COUNT(*) FROM instances WHERE id='target'`, `SELECT COUNT(*) FROM jobs WHERE target_id='target'`, `SELECT COUNT(*) FROM job_logs`, `SELECT COUNT(*) FROM audit_logs WHERE target_id='target'`} {
		var n int
		if err = s.db.QueryRow(query).Scan(&n); err != nil || n != 0 {
			t.Fatal(query, n, err)
		}
	}
	for _, id := range []string{"configured-default", "other"} {
		if _, err = s.GetInstance(ctx, id); err != nil {
			t.Fatal("other world removed", err)
		}
	}
	for _, table := range []string{"save_identities", "player_roster", "player_events", "control_commands", "restart_schedules"} {
		var n int
		if err = s.db.QueryRow(`SELECT COUNT(*) FROM ` + table + ` WHERE instance_id='target'`).Scan(&n); err != nil || n != 0 {
			t.Fatal(table, n, err)
		}
	}
	record, err = s.GetInstanceDeletion(ctx, "target")
	if err != nil || !record.Completed || record.Plan != "" {
		t.Fatal(record, err)
	}
	if _, err = s.CreateInstance(ctx, CreateInstanceParams{ID: "target", DriverID: DefaultDriverID, Name: "reuse", DataDir: "/instances/reuse", State: InstanceStateStopped}); err == nil {
		t.Fatal("deleted ID reused")
	}
}
