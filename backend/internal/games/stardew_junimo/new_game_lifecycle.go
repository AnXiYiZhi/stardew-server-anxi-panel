package stardew_junimo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
)

const (
	newGameDefaultObservationTimeout = 25 * time.Minute
	newGameControlDurabilityTimeout  = 10 * time.Minute
	newGameDurableSaveTimeout        = 5 * time.Minute
	newGameDiskDurabilityTimeout     = 5 * time.Minute
)

// sendNewGameCommand has one durable success path:
// candidate -> exact Control SaveLoaded/customization -> same-ID GameLoop.Saved
// -> stable main + SaveGameInfo XML -> profile/transaction commit.
func (r *lifecycleRunner) sendNewGameCommand(ctx context.Context, jobCtx *jobs.Context, tx *newGameTransaction) error {
	if tx == nil {
		return &NewGameTransactionError{Code: "new_game_transaction_missing", Message: "新建存档事务不存在"}
	}
	if err := tx.assertOwner(); err != nil {
		return err
	}
	if tx.record.DiskVerifiedAt != nil && tx.record.CandidateSave != "" {
		_, _ = jobCtx.Info(ctx, "磁盘耐久证据已持久化，继续完成 Mod profile 与事务提交。")
		return r.finalizeDurableNewGame(ctx, jobCtx, tx, tx.record.CandidateSave)
	}
	if err := tx.waitForRuntimeFarmCatalog(ctx, r.newGameCatalogTimeout, r.newGamePollInterval); err != nil {
		return err
	}
	_, _ = jobCtx.Info(ctx, "运行时农场目录已通过 transactionId、Mod 指纹和 FarmType 校验。")

	pollInterval := r.newGamePollInterval
	if pollInterval <= 0 {
		pollInterval = 3 * time.Second
	}

	commandErr := error(nil)
	if !tx.record.CommandCalled {
		evidence, err := tx.observeNewGameProgress()
		if err != nil {
			return &NewGameTransactionError{Code: "new_game_progress_read_failed", Message: "读取新建存档进展证据失败", Cause: err}
		}
		if evidence.Ambiguous {
			return markNewGameAmbiguous(tx, evidence.NewSaveDirs)
		}
		if evidence.Observed {
			_, _ = jobCtx.Info(ctx, "已观察到当前事务的创建进展；禁止提交第二个创建写入者。")
		} else if tx.record.CreationWriter == newGameCreationWriterStartup {
			_, _ = jobCtx.Info(ctx, "当前事务固定使用 Junimo 启动建档；不会调用 /newgame。")
		} else if tx.record.CreationWriter == newGameCreationWriterHTTP {
			progressDuringBaseline, err := r.waitForHTTPNewGameBaseline(ctx, tx, pollInterval)
			if err != nil {
				return err
			}
			if progressDuringBaseline {
				_, _ = jobCtx.Info(ctx, "等待旧存档稳定期间已出现创建进展；跳过 /newgame。")
			} else {
				if err := r.waitForNewGameAPI(ctx, pollInterval); err != nil {
					return err
				}
				// Close the last pre-POST window. The HTTP writer is selected only
				// after a complete old save reached a stable SaveLoaded baseline.
				evidence, err = tx.observeNewGameProgress()
				if err != nil {
					return &NewGameTransactionError{Code: "new_game_progress_read_failed", Message: "调用 /newgame 前无法复核创建进展", Cause: err}
				}
				if evidence.Ambiguous {
					return markNewGameAmbiguous(tx, evidence.NewSaveDirs)
				}
				if evidence.Observed {
					_, _ = jobCtx.Info(ctx, "调用前最后检查发现创建进展；跳过 /newgame。")
				} else {
					commandErr = r.postNewGameOnce(ctx, jobCtx, tx)
				}
			}
		} else {
			return &NewGameTransactionError{Code: "new_game_recovery_required", Message: "事务没有有效的固定创建写入者"}
		}
	} else {
		_, _ = jobCtx.Warn(ctx, "事务已持久化 /newgame intent；只观察原请求结果，绝不生成新 intent。")
	}

	if tx.record.CandidateSave == "" {
		if err := tx.mark(newGameStateObserving); err != nil {
			return &NewGameTransactionError{Code: "new_game_state_write_failed", Message: "记录结果观察状态失败", Cause: err}
		}
	}
	candidate, err := r.waitForNewGameCandidate(ctx, tx, pollInterval, commandErr)
	if err != nil {
		return err
	}
	if err := tx.bindTargetSave(candidate); err != nil {
		return err
	}
	_, _ = jobCtx.Info(ctx, fmt.Sprintf("已将单一候选 %s 持久绑定到 Control marker。", candidate))

	controlStatus, err := waitForNewGameControlDurability(ctx, tx.dataDir, tx.record.TransactionID, candidate, tx.record.Config,
		newGameControlDurabilityWaitOptions{
			Timeout:        newGameGateTimeout(r.newGameControlGateTimeout, newGameControlDurabilityTimeout),
			PollInterval:   minNewGamePollInterval(pollInterval),
			FreshAfter:     tx.record.CreatedAt,
			PreviousSaveID: tx.record.InitialGameloaderSave,
		})
	if err != nil {
		return err
	}
	if err := tx.recordControlDurability(controlStatus); err != nil {
		return &NewGameTransactionError{Code: "new_game_state_write_failed", Message: "持久化 SaveLoaded 与角色内存复核证据失败", Cause: err}
	}
	_, _ = jobCtx.Info(ctx, "Control 已确认 save-loaded，且角色定制在目标存档内存中逐字段一致。")

	commandID, err := tx.ensureDurableSaveCommandID()
	if err != nil {
		return &NewGameTransactionError{Code: "new_game_state_write_failed", Message: "持久预留 save-now commandId 失败", Cause: err}
	}
	freshAfter := controlStatus.UpdatedAt
	if tx.record.SaveLoadedAt != nil {
		freshAfter = *tx.record.SaveLoadedAt
	}
	outcome, err := submitAndWaitForNewGameDurableSave(ctx, tx.dataDir, commandID, tx.record.TransactionID, candidate,
		newGameDurableSaveOptions{
			Timeout:      newGameGateTimeout(r.newGameSaveGateTimeout, newGameDurableSaveTimeout),
			PollInterval: minNewGamePollInterval(pollInterval),
			FreshAfter:   freshAfter,
			Publish: func(_ string, commandID, transactionID, saveID string) error {
				return tx.publishDurableSaveCommand(commandID, transactionID, saveID)
			},
		})
	if err != nil {
		return err
	}
	if err := tx.recordDurableSaved(outcome); err != nil {
		return &NewGameTransactionError{Code: "new_game_state_write_failed", Message: "持久化 GameLoop.Saved 结果失败", Cause: err}
	}
	_, _ = jobCtx.Info(ctx, fmt.Sprintf("同一 save-now commandId %s 已收到精确 GameLoop.Saved 成功结果。", commandID))

	expectedFarmType := tx.record.ResolvedFarmType
	if expectedFarmType == "" {
		expectedFarmType = tx.record.RequestedFarmType
	}
	disk, err := waitForNewGameDiskDurability(ctx, tx.dataDir, candidate, tx.record.Config,
		newGameDiskDurabilityWaitOptions{
			Timeout:          newGameGateTimeout(r.newGameDiskGateTimeout, newGameDiskDurabilityTimeout),
			PollInterval:     minNewGamePollInterval(pollInterval),
			ExpectedFarmType: expectedFarmType,
		})
	if err != nil {
		return err
	}
	if err := r.verifyNewGameIdentityConvergence(tx, candidate); err != nil {
		return err
	}
	if err := tx.recordDiskDurability(disk); err != nil {
		return &NewGameTransactionError{Code: "new_game_state_write_failed", Message: "持久化双 XML SHA-256 证据失败", Cause: err}
	}
	_, _ = jobCtx.Info(ctx, "主存档与 SaveGameInfo 已稳定，磁盘角色字段、农场类型和双 SHA-256 均通过。")
	return r.finalizeDurableNewGame(ctx, jobCtx, tx, candidate)
}

func (r *lifecycleRunner) waitForHTTPNewGameBaseline(ctx context.Context, tx *newGameTransaction, pollInterval time.Duration) (bool, error) {
	expected := strings.TrimSpace(tx.record.ActiveSave)
	if expected == "" {
		expected = strings.TrimSpace(tx.record.InitialGameloaderSave)
	}
	if expected == "" {
		return false, &NewGameTransactionError{Code: "new_game_http_baseline_missing", Message: "HTTP 创建写入者缺少完整旧存档基线"}
	}
	timeout := r.newGameAPIReadyTimeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	stable := 0
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		evidence, err := tx.observeNewGameProgress()
		if err != nil {
			return false, &NewGameTransactionError{Code: "new_game_progress_read_failed", Message: "等待旧存档基线时无法读取创建进展", Cause: err}
		}
		if evidence.Ambiguous {
			return false, markNewGameAmbiguous(tx, evidence.NewSaveDirs)
		}
		if evidence.Observed {
			return true, nil
		}
		status := evidence.ControlStatus
		fresh := status.UpdatedAt != nil && !status.UpdatedAt.Before(tx.record.CreatedAt)
		if status.Present && fresh && status.State == "save-loaded" && status.SaveID == expected && status.TransactionID == tx.record.TransactionID && !status.CreationObserved {
			stable++
			if stable >= 2 {
				return false, nil
			}
		} else {
			stable = 0
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-deadline.C:
			return false, &NewGameTransactionError{Code: "new_game_http_baseline_timeout", Message: "旧存档未在期限内达到稳定的 save-loaded 基线；不会提交 /newgame"}
		case <-ticker.C:
		}
	}
}

func (r *lifecycleRunner) waitForNewGameAPI(ctx context.Context, pollInterval time.Duration) error {
	timeout := r.newGameAPIReadyTimeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		result, err := r.lifecycle.ComposeExecPipe(requestCtx, r.instance.DataDir, "server", "", "curl", "-sf", "http://localhost:8080/status")
		cancel()
		if err == nil && result.ExitCode == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return &NewGameTransactionError{Code: "new_game_api_not_ready", Message: "服务器 API 在期限内未就绪；不会提交 /newgame"}
		case <-ticker.C:
		}
	}
}

func (r *lifecycleRunner) postNewGameOnce(ctx context.Context, jobCtx *jobs.Context, tx *newGameTransaction) error {
	if err := tx.markCommandCalled(); err != nil {
		return &NewGameTransactionError{Code: "new_game_state_write_failed", Message: "持久化 /newgame intent 失败", Cause: err}
	}
	timeout := r.newGameCommandTimeout
	if timeout <= 0 {
		timeout = 4 * time.Minute
	}
	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	result, err := r.lifecycle.ComposeExecPipe(commandCtx, r.instance.DataDir, "server", "", "curl", "-sf", "-X", "POST", "-H", "Content-Type: application/json", "-d", "{}", "http://localhost:8080/newgame")
	timedOut := errors.Is(commandCtx.Err(), context.DeadlineExceeded)
	cancel()
	if err != nil {
		if timedOut {
			_, _ = jobCtx.Warn(ctx, "创建请求响应超时；不会重提，继续观察原事务进展。")
			return context.DeadlineExceeded
		}
		_, _ = jobCtx.Warn(ctx, fmt.Sprintf("创建请求未正常返回（%s）；不会重提，继续观察原事务进展。", paneldocker.RedactString(err.Error())))
		return err
	}
	if result.ExitCode != 0 {
		err = fmt.Errorf("newgame request exited with code %d", result.ExitCode)
		_, _ = jobCtx.Warn(ctx, "创建请求返回失败；不会重提，继续观察原事务进展。")
		return err
	}
	_, _ = jobCtx.Info(ctx, "唯一 /newgame 请求已返回，正在观察事务绑定的候选存档。")
	return nil
}

func (r *lifecycleRunner) waitForNewGameCandidate(ctx context.Context, tx *newGameTransaction, pollInterval time.Duration, commandErr error) (string, error) {
	if tx.record.CandidateSave != "" {
		return tx.record.CandidateSave, nil
	}
	timeout := r.newGameObservationTimeout
	if timeout <= 0 {
		timeout = newGameDefaultObservationTimeout
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		evidence, err := tx.observeNewGameProgress()
		if err != nil {
			return "", &NewGameTransactionError{Code: "new_game_progress_read_failed", Message: "读取新建存档进展证据失败", Cause: err}
		}
		if evidence.Ambiguous {
			return "", markNewGameAmbiguous(tx, evidence.NewSaveDirs)
		}
		if evidence.SaveName != "" {
			return evidence.SaveName, nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-deadline.C:
			if tx.record.CommandCalled || evidence.Observed {
				cause := commandErr
				if cause == nil {
					cause = errors.New("creation progress did not converge to a unique save id")
				}
				tx.setFailure(newGameStateUnknown, "new_game_outcome_unknown", cause)
				return "", &NewGameTransactionError{Code: "new_game_outcome_unknown", Message: "创建进展未在期限内收敛到唯一存档；保留现场且禁止重提", Cause: cause}
			}
			return "", &NewGameTransactionError{Code: "new_game_save_not_found", Message: "固定的 Junimo 启动建档流程未产生任何创建进展"}
		case <-ticker.C:
		}
	}
}

func (r *lifecycleRunner) verifyNewGameIdentityConvergence(tx *newGameTransaction, candidate string) error {
	evidence, err := tx.observeNewGameProgress()
	if err != nil {
		return &NewGameTransactionError{Code: "new_game_progress_read_failed", Message: "最终复核创建身份失败", Cause: err}
	}
	if evidence.Ambiguous || evidence.SaveName != candidate || len(evidence.NewSaveDirs) != 1 || evidence.NewSaveDirs[0] != candidate {
		return &NewGameTransactionError{Code: "new_game_identity_mismatch", Message: "Control、loader 与唯一新目录未收敛到同一目标存档"}
	}
	status, err := readNewGameControlDurabilityStatus(tx.dataDir)
	if err != nil {
		return &NewGameTransactionError{Code: "new_game_control_status_unreadable", Message: "最终复核时无法读取 Control 目标身份", Cause: err}
	}
	if status.State != "save-loaded" || status.NewGameTransactionID != tx.record.TransactionID ||
		status.SaveID != candidate || !status.NewGameCreationObserved ||
		status.CustomizationTransactionID != tx.record.TransactionID || status.CustomizationSaveID != candidate {
		return &NewGameTransactionError{Code: "new_game_identity_mismatch", Message: "Control、loader 与唯一新目录未收敛到同一目标存档"}
	}
	loader := readGameloaderSaveName(gameloaderPath(tx.dataDir))
	if loader != candidate {
		if uniqueNumericSuffixCandidate(loader, evidence.NewSaveDirs) != candidate {
			return &NewGameTransactionError{Code: "new_game_gameloader_mismatch", Message: "gameloader 未精确指向当前事务的唯一目标存档"}
		}
		if err := tx.assertOwner(); err != nil {
			return err
		}
		if err := writeGameloaderPointer(tx.dataDir, candidate); err != nil {
			return &NewGameTransactionError{Code: "new_game_gameloader_repair_failed", Message: "修复新存档指针失败", Cause: err}
		}
	}
	return nil
}

func (r *lifecycleRunner) finalizeDurableNewGame(ctx context.Context, jobCtx *jobs.Context, tx *newGameTransaction, candidate string) error {
	if err := tx.assertOwner(); err != nil {
		return err
	}
	profileKeys := []string{}
	if tx.record.ModSelection != nil {
		profileKeys = append(profileKeys, tx.record.EnabledModKeys...)
	}
	commitProfile := r.commitNewGameModProfile
	if commitProfile == nil {
		commitProfile = EnsureNewSaveModProfile
	}
	if err := commitProfile(tx.dataDir, candidate, profileKeys); err != nil {
		tx.record.CreatedSave = candidate
		tx.setFailure(newGameStateProfilePending, "mod_profile_commit_failed", err)
		return &NewGameTransactionError{Code: "mod_profile_commit_failed", Message: "耐久存档已保留，但 Mod profile 提交失败；恢复时只会重试 profile commit", Cause: err}
	}
	if err := tx.complete(candidate); err != nil {
		return &NewGameTransactionError{Code: "new_game_state_write_failed", Message: "记录新存档成功状态失败", Cause: err}
	}
	if err := tx.releaseOwner(); err != nil {
		// The transaction is already durably terminal. Leaving the fenced owner
		// in place is safe; the next explicit start cleans this terminal lease.
		_, _ = jobCtx.Warn(ctx, "新存档已成功提交，但持久 owner 暂未清理；下次操作会安全重试清理。")
	}
	_, _ = jobCtx.Info(ctx, fmt.Sprintf("新存档已完成四段耐久化验收：%s（%s）", candidate, tx.record.RequestedFarmType))
	return nil
}

func markNewGameAmbiguous(tx *newGameTransaction, dirs []string) error {
	cause := fmt.Errorf("conflicting creation evidence: %s", strings.Join(dirs, ", "))
	tx.record.Stage = newGameStateAmbiguous
	tx.record.Result = "ambiguous"
	tx.record.ErrorCode = "new_game_ambiguous"
	tx.record.ErrorMessage = cause.Error()
	if err := tx.persist(); err != nil {
		return &NewGameTransactionError{Code: "new_game_state_write_failed", Message: "持久化新建存档歧义状态失败", Cause: err}
	}
	return &NewGameTransactionError{Code: "new_game_ambiguous", Message: "检测到多个或冲突的新建存档证据；保留现场且禁止重提", Cause: cause}
}

func minNewGamePollInterval(configured time.Duration) time.Duration {
	if configured <= 0 || configured > time.Second {
		return time.Second
	}
	return configured
}

func newGameGateTimeout(configured, fallback time.Duration) time.Duration {
	if configured > 0 {
		return configured
	}
	return fallback
}

func refreshPendingNewGameMarker(tx *newGameTransaction) error {
	if tx.record.DiskVerifiedAt != nil {
		return nil
	}
	if err := tx.assertOwner(); err != nil {
		return err
	}
	raw, err := os.ReadFile(newGamePendingPath(tx.dataDir))
	if err != nil {
		return &NewGameTransactionError{Code: "new_game_recovery_required", Message: "恢复事务时 pending marker 缺失", Cause: err}
	}
	var marker newGamePendingMarker
	if err := jsonUnmarshal(string(raw), &marker); err != nil || marker.SchemaVersion != newGameMarkerSchemaVersion || marker.TransactionID != tx.record.TransactionID || marker.RequestedFarmType != tx.record.RequestedFarmType || !strings.EqualFold(marker.State, "pending") {
		return &NewGameTransactionError{Code: "new_game_recovery_required", Message: "恢复事务时 pending marker 身份无效", Cause: err}
	}
	if marker.TargetSaveID != "" && tx.record.CandidateSave != "" && marker.TargetSaveID != tx.record.CandidateSave {
		return &NewGameTransactionError{Code: "new_game_recovery_required", Message: "恢复事务时 marker 目标与事务候选不一致"}
	}
	marker.ExpiresAt = time.Now().UTC().Add(newGameMarkerTTL)
	payload, err := marshalJSON(marker)
	if err != nil {
		return err
	}
	return tx.writeJSON(newGamePendingPath(tx.dataDir), payload, 0o644)
}
