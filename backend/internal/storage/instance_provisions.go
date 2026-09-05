package storage

import (
	"context"
	"database/sql"
	"errors"
)

type InstanceProvision struct{ InstanceID, TemplateID, Token string }

func (s *Store) InstanceProvisionOwner(ctx context.Context, id string) (string, error) {
	var token string
	err := s.db.QueryRowContext(ctx, `SELECT token FROM instance_provisions WHERE instance_id=? OR template_id=? LIMIT 1`, id, id).Scan(&token)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return token, err
}

func (s *Store) GetInstanceProvision(ctx context.Context, id string) (InstanceProvision, error) {
	var p InstanceProvision
	err := s.db.QueryRowContext(ctx, `SELECT instance_id,template_id,token FROM instance_provisions WHERE instance_id=?`, id).Scan(&p.InstanceID, &p.TemplateID, &p.Token)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return p, err
}

func (s *Store) BeginInstanceProvision(ctx context.Context, p InstanceProvision) error {
	if p.InstanceID == "" || p.TemplateID == "" || p.InstanceID == p.TemplateID || p.Token == "" {
		return ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollback(tx)
	var busy int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM jobs WHERE status IN ('queued','running') AND (target_type!='instance' OR target_id IN (?,?))`, p.InstanceID, p.TemplateID).Scan(&busy); err != nil {
		return err
	}
	if busy != 0 {
		return ErrConflict
	}
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM instances WHERE id=? AND driver_phase='instance_provisioning'`, p.InstanceID).Scan(&busy); err != nil {
		return err
	}
	if busy != 1 {
		return ErrConflict
	}
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM instance_provisions WHERE instance_id IN (?,?) OR template_id IN (?,?)`, p.InstanceID, p.TemplateID, p.InstanceID, p.TemplateID).Scan(&busy); err != nil {
		return err
	}
	if busy != 0 {
		return ErrConflict
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO instance_provisions(instance_id,template_id,token) VALUES (?,?,?)`, p.InstanceID, p.TemplateID, p.Token); err != nil {
		return err
	}
	return tx.Commit()
}

// Finish is atomic with journal removal, so bootstrap never rolls back a
// successfully published world, even if the HTTP response was lost.
func (s *Store) FinishInstanceProvision(ctx context.Context, p InstanceProvision, publish bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollback(tx)
	var n int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM instance_provisions WHERE instance_id=? AND token=?`, p.InstanceID, p.Token).Scan(&n); err != nil {
		return err
	}
	if n != 1 {
		return ErrConflict
	}
	if publish {
		_, err = tx.ExecContext(ctx, `UPDATE instances SET state='save_required',driver_phase='instance_ready',state_message='世界实例已创建，请创建或导入存档。',driver_payload='{}',updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id=?`, p.InstanceID)
	} else {
		_, err = tx.ExecContext(ctx, `DELETE FROM instances WHERE id=?`, p.InstanceID)
	}
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM instance_provisions WHERE instance_id=? AND token=?`, p.InstanceID, p.Token); err != nil {
		return err
	}
	return tx.Commit()
}
