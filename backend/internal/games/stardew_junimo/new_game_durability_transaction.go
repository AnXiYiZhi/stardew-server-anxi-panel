package stardew_junimo

import (
	"fmt"
	"time"
)

// recordControlDurability persists the first accepted SaveLoaded/customization
// gate. Keeping the original time across recovery lets the exact same save-now
// result remain valid after a Panel or game-process restart.
func (tx *newGameTransaction) recordControlDurability(status newGameControlDurabilityStatus) error {
	if err := tx.assertOwner(); err != nil {
		return err
	}
	if status.NewGameTransactionID != tx.record.TransactionID || status.SaveID != tx.record.CandidateSave ||
		status.CustomizationVerifiedAt == nil {
		return &NewGameTransactionError{Code: "new_game_control_identity_mismatch", Message: "无法把不属于当前事务和目标存档的 Control 门禁写入事务"}
	}
	if tx.record.SaveLoadedAt != nil || tx.record.CustomizationVerifiedAt != nil {
		if tx.record.SaveLoadedAt == nil || tx.record.CustomizationVerifiedAt == nil {
			return &NewGameTransactionError{Code: "new_game_recovery_required", Message: "事务中的 Control 耐久化时间证据不完整"}
		}
		return nil
	}
	loadedAt := status.UpdatedAt.UTC()
	verifiedAt := status.CustomizationVerifiedAt.UTC()
	tx.record.SaveLoadedAt = &loadedAt
	tx.record.CustomizationVerifiedAt = &verifiedAt
	return tx.persist()
}

// ensureDurableSaveCommandID records the irreversible command intent before a
// command file is visible. Recovery always observes or republishes this same ID.
func (tx *newGameTransaction) ensureDurableSaveCommandID() (string, error) {
	if err := tx.assertOwner(); err != nil {
		return "", err
	}
	if tx.record.DurableSaveCommandID != "" {
		if !validCommandID(tx.record.DurableSaveCommandID) {
			return "", &NewGameTransactionError{Code: "new_game_recovery_required", Message: "事务中的 save-now commandId 无效"}
		}
		return tx.record.DurableSaveCommandID, nil
	}
	commandID, err := newGameRandomHex(16)
	if err != nil {
		return "", fmt.Errorf("generate durable save command id: %w", err)
	}
	tx.record.DurableSaveCommandID = commandID
	if err := tx.persist(); err != nil {
		tx.record.DurableSaveCommandID = ""
		return "", err
	}
	return commandID, nil
}

func (tx *newGameTransaction) publishDurableSaveCommand(commandID, transactionID, saveID string) error {
	if err := tx.assertOwner(); err != nil {
		return err
	}
	if commandID != tx.record.DurableSaveCommandID || transactionID != tx.record.TransactionID || saveID != tx.record.CandidateSave {
		return &NewGameTransactionError{Code: "new_game_durable_save_identity_invalid", Message: "save-now 发布身份与持久事务不一致"}
	}
	if err := writePanelCommandWithID(tx.dataDir, commandID, "save-now", map[string]string{
		"transactionId": transactionID,
		"saveId":        saveID,
	}); err != nil {
		return err
	}
	if tx.record.DurableSaveCommandPublishedAt == nil {
		now := time.Now().UTC()
		tx.record.DurableSaveCommandPublishedAt = &now
		if err := tx.persist(); err != nil {
			return err
		}
	}
	return nil
}

func (tx *newGameTransaction) recordDurableSaved(outcome CommandOutcome) error {
	if err := tx.assertOwner(); err != nil {
		return err
	}
	if outcome.CommandID != tx.record.DurableSaveCommandID || outcome.Status != CommandStatusSucceeded || outcome.ErrorCode != "ok" {
		return &NewGameTransactionError{Code: "new_game_durable_save_evidence_mismatch", Message: "无法持久化不匹配的 GameLoop.Saved 结果"}
	}
	if tx.record.DurableGameLoopSavedAt == nil {
		savedAt := outcome.UpdatedAt.UTC()
		tx.record.DurableGameLoopSavedAt = &savedAt
		return tx.persist()
	}
	return nil
}

func (tx *newGameTransaction) recordDiskDurability(evidence newGameDiskDurabilityEvidence) error {
	if err := tx.assertOwner(); err != nil {
		return err
	}
	if !isValidNewGameSHA256(evidence.MainSHA256) || !isValidNewGameSHA256(evidence.SaveGameInfoSHA256) {
		return &NewGameTransactionError{Code: "new_game_disk_evidence_invalid", Message: "新建存档磁盘 SHA-256 证据无效"}
	}
	tx.record.MainSaveSHA256 = evidence.MainSHA256
	tx.record.SaveGameInfoSHA256 = evidence.SaveGameInfoSHA256
	now := time.Now().UTC()
	tx.record.DiskVerifiedAt = &now
	return tx.persist()
}
