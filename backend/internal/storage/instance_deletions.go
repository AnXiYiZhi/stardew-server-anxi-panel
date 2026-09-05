package storage

import (
	"context"
	"database/sql"
	"errors"
)

// InstanceDeletion is a durable resource plan, independent of jobs that are
// themselves erased at completion. An empty plan is the completed tombstone.
type InstanceDeletion struct {
	Plan      string
	Completed bool
}

func (s *Store) GetInstanceDeletion(ctx context.Context, id string) (InstanceDeletion, error) {
	var result InstanceDeletion
	err := s.db.QueryRowContext(ctx, `SELECT plan, completed FROM instance_deletions WHERE instance_id = ?`, id).Scan(&result.Plan, &result.Completed)
	if errors.Is(err, sql.ErrNoRows) {
		return result, ErrNotFound
	}
	return result, err
}

func (s *Store) BeginInstanceDeletion(ctx context.Context, id, defaultID, plan string) error {
	if id == "" || id == defaultID || defaultID == "" || plan == "" {
		return ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollback(tx)
	var busy int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM jobs WHERE status IN ('queued','running') AND (target_type != 'instance' OR target_id = ?)`, id).Scan(&busy); err != nil {
		return err
	}
	if busy != 0 {
		return ErrConflict
	}
	result, err := tx.ExecContext(ctx, `UPDATE instances SET state='error', driver_phase='instance_deleting', state_message='世界删除尚未完成，请重试彻底删除。' WHERE id=? AND state NOT IN ('running','starting')`, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrConflict
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO instance_deletions(instance_id,plan) VALUES (?,?)`, id, plan); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CompleteInstanceDeletion(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollback(tx)
	var owned int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM instance_deletions WHERE instance_id=?`, id).Scan(&owned); err != nil {
		return err
	}
	if owned != 1 {
		return ErrConflict
	}
	for _, query := range []string{
		`DELETE FROM instance_state WHERE instance_id=?`,
		`DELETE FROM audit_logs WHERE target_type='job' AND target_id IN (SELECT id FROM jobs WHERE target_type='instance' AND target_id=?)`,
		`DELETE FROM jobs WHERE target_type='instance' AND target_id=?`,
		`DELETE FROM audit_logs WHERE target_type='instance' AND target_id=?`,
		`DELETE FROM instances WHERE id=?`,
		`UPDATE instance_deletions SET plan='',completed=1 WHERE instance_id=?`,
	} {
		if _, err = tx.ExecContext(ctx, query, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
