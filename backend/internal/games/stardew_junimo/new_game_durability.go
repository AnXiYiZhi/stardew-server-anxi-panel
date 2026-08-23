package stardew_junimo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

const maxNewGameControlStatusBytes = 256 * 1024

type newGameCoreCustomization struct {
	FarmerName    string           `json:"farmerName"`
	FarmName      string           `json:"farmName"`
	FavoriteThing string           `json:"favoriteThing"`
	Gender        string           `json:"gender"`
	PetType       string           `json:"petType"`
	PetBreed      string           `json:"petBreed"`
	Skin          *int             `json:"skin,omitempty"`
	Hair          *int             `json:"hair,omitempty"`
	Shirt         string           `json:"shirt,omitempty"`
	Pants         string           `json:"pants,omitempty"`
	Accessory     *int             `json:"accessory,omitempty"`
	EyeColor      *newGameRGBColor `json:"eyeColor,omitempty"`
	HairColor     *newGameRGBColor `json:"hairColor,omitempty"`
	PantsColor    *newGameRGBColor `json:"pantsColor,omitempty"`
	IsCustomized  bool             `json:"isCustomized"`
}

type newGameRGBColor struct {
	R int `json:"r"`
	G int `json:"g"`
	B int `json:"b"`
}

type newGamePlayersDurability struct {
	UpdatedAt time.Time `json:"updatedAt"`
	SaveID    string    `json:"saveId"`
	Players   []struct {
		Name   string `json:"name"`
		IsHost bool   `json:"isHost"`
	} `json:"players"`
}

// newGameControlDurabilityStatus is the frozen in-memory evidence emitted by
// Control on SaveLoaded. It is deliberately separate from the lightweight
// progress snapshot: success requires the complete transaction-bound payload.
type newGameControlDurabilityStatus struct {
	State                       string                    `json:"state"`
	SaveID                      string                    `json:"saveId"`
	UpdatedAt                   time.Time                 `json:"updatedAt"`
	NewGameTransactionID        string                    `json:"newGameTransactionId"`
	NewGameCreationObserved     bool                      `json:"newGameCreationObserved"`
	CustomizationApplied        bool                      `json:"customizationApplied"`
	CustomizationVerified       bool                      `json:"customizationVerified"`
	CustomizationTransactionID  string                    `json:"customizationTransactionId"`
	CustomizationSaveID         string                    `json:"customizationSaveId"`
	CustomizationVerifiedAt     *time.Time                `json:"customizationVerifiedAt"`
	Customization               *newGameCoreCustomization `json:"customization"`
	CustomizationMismatches     []string                  `json:"customizationMismatches"`
	FarmCaveChoiceApplied       bool                      `json:"farmCaveChoiceApplied"`
	FarmCaveChoiceVerified      bool                      `json:"farmCaveChoiceVerified"`
	FarmCaveChoiceTransactionID string                    `json:"farmCaveChoiceTransactionId"`
	FarmCaveChoiceSaveID        string                    `json:"farmCaveChoiceSaveId"`
	FarmCaveChoiceVerifiedAt    *time.Time                `json:"farmCaveChoiceVerifiedAt"`
	FarmCaveChoice              *newGameFarmCaveChoice    `json:"farmCaveChoice"`
	FarmCaveChoiceAttempt       *newGameFarmCaveChoice    `json:"farmCaveChoiceAttempt"`
	FarmCaveChoiceErrorCode     string                    `json:"farmCaveChoiceErrorCode"`
}

type newGameFarmCaveChoice struct {
	RequestedChoice        string `json:"requestedChoice"`
	ActualChoice           int    `json:"actualChoice"`
	ChoiceEventSeen        bool   `json:"choiceEventSeen"`
	MushroomHouseReady     bool   `json:"mushroomHouseReady"`
	MushroomObjectsPresent bool   `json:"mushroomObjectsPresent"`
	MushroomBoxCount       int    `json:"mushroomBoxCount"`
	DehydratorCount        int    `json:"dehydratorCount"`
}

type newGameControlDurabilityWaitOptions struct {
	Timeout      time.Duration
	PollInterval time.Duration
	FreshAfter   time.Time
	// PreviousSaveID is the complete active save loaded as the HTTP writer's
	// pre-command baseline. Its SaveLoaded snapshot may remain visible for a
	// short time after a new candidate appears; only this exact old identity is
	// pending rather than a terminal target mismatch.
	PreviousSaveID string
}

type newGameDurableSaveOptions struct {
	Timeout      time.Duration
	PollInterval time.Duration
	// FreshAfter should be the accepted SaveLoaded status UpdatedAt. A caller
	// must not let an older same-ID result satisfy the post-customization save.
	FreshAfter time.Time
	Publish    func(dataDir, commandID, transactionID, saveID string) error
	GetOutcome func(dataDir, commandID string) (CommandOutcome, error)
}

type newGameDiskDurabilityEvidence struct {
	MainSHA256         string
	SaveGameInfoSHA256 string
	SaveGameInfoPath   string
}

type newGameDiskDurabilityWaitOptions struct {
	Timeout          time.Duration
	PollInterval     time.Duration
	ExpectedFarmType string
}

func readNewGameControlDurabilityStatus(dataDir string) (newGameControlDurabilityStatus, error) {
	var status newGameControlDurabilityStatus
	raw, err := readRuntimeStatusFile(filepath.Join(controlDir(dataDir), "status.json"))
	if err != nil {
		return status, err
	}
	if len(raw) == 0 || len(raw) > maxNewGameControlStatusBytes {
		return status, &NewGameTransactionError{
			Code:    "new_game_control_status_invalid",
			Message: "Control 状态文件为空或超过大小限制",
		}
	}
	if err := json.Unmarshal(raw, &status); err != nil {
		return status, &NewGameTransactionError{
			Code:    "new_game_control_status_invalid",
			Message: "Control 状态文件不是有效 JSON",
			Cause:   err,
		}
	}
	return status, nil
}

// inspectNewGameControlDurability returns ready=false for evidence that can
// legitimately advance (missing, stale, or a pre-SaveLoaded state). Once a
// fresh SaveLoaded snapshot exists, every identity and customization mismatch
// is terminal so callers never save an incorrectly customized world.
func inspectNewGameControlDurability(
	dataDir string,
	transactionID string,
	saveID string,
	cfg registry.NewGameConfig,
	freshAfter time.Time,
) (newGameControlDurabilityStatus, bool, error) {
	status, err := readNewGameControlDurabilityStatus(dataDir)
	if errors.Is(err, os.ErrNotExist) {
		return status, false, nil
	}
	if err != nil {
		var txErr *NewGameTransactionError
		if errors.As(err, &txErr) {
			return status, false, err
		}
		return status, false, &NewGameTransactionError{
			Code:    "new_game_control_status_unreadable",
			Message: "无法读取 Control 状态文件",
			Cause:   err,
		}
	}
	if status.UpdatedAt.IsZero() {
		return status, false, &NewGameTransactionError{
			Code:    "new_game_control_status_invalid",
			Message: "Control 状态缺少 updatedAt",
		}
	}
	if !freshAfter.IsZero() && status.UpdatedAt.Before(freshAfter) {
		return status, false, nil
	}
	if status.State == "save-customization-invalid" && status.NewGameTransactionID == transactionID && status.SaveID == saveID {
		message := "Control 已明确报告目标存档的角色定制内存复核失败"
		if len(status.CustomizationMismatches) > 0 {
			message += "（不匹配字段：" + strings.Join(status.CustomizationMismatches, ", ") + "）"
		}
		return status, false, &NewGameTransactionError{
			Code:    "new_game_control_customization_mismatch",
			Message: message,
		}
	}
	if status.State == "save-cave-choice-invalid" && status.NewGameTransactionID == transactionID && status.SaveID == saveID {
		message := "Control 已明确报告目标存档的农场山洞选择复核失败"
		if status.FarmCaveChoiceErrorCode != "" {
			message += "（" + status.FarmCaveChoiceErrorCode + "）"
		}
		if status.FarmCaveChoiceAttempt != nil {
			message += fmt.Sprintf("；实际 caveChoice=%d，事件 65=%t，蘑菇设施=%t",
				status.FarmCaveChoiceAttempt.ActualChoice,
				status.FarmCaveChoiceAttempt.ChoiceEventSeen,
				status.FarmCaveChoiceAttempt.MushroomHouseReady)
		}
		return status, false, &NewGameTransactionError{
			Code:    "new_game_control_farm_cave_mismatch",
			Message: message,
		}
	}
	if status.State != "save-loaded" {
		return status, false, nil
	}
	if status.NewGameTransactionID != transactionID || status.SaveID != saveID {
		return status, false, &NewGameTransactionError{
			Code:    "new_game_control_identity_mismatch",
			Message: "Control 的 SaveLoaded 状态不属于当前新建存档事务或目标存档",
		}
	}
	if !status.NewGameCreationObserved {
		return status, false, nil
	}
	if !status.CustomizationApplied || !status.CustomizationVerified || status.CustomizationVerifiedAt == nil {
		return status, false, nil
	}
	if status.CustomizationVerifiedAt.Before(freshAfter) || status.CustomizationVerifiedAt.After(status.UpdatedAt) {
		return status, false, &NewGameTransactionError{
			Code:    "new_game_control_customization_unconfirmed",
			Message: "Control 的角色定制复核时间不属于当前 SaveLoaded 状态",
		}
	}
	if status.CustomizationTransactionID != transactionID || status.CustomizationSaveID != saveID {
		return status, false, &NewGameTransactionError{
			Code:    "new_game_control_customization_identity_mismatch",
			Message: "Control 的角色定制快照未冻结到当前事务和目标存档",
		}
	}
	expected := expectedNewGameCoreCustomization(cfg)
	if status.Customization == nil || !newGameCustomizationMatches(expected, *status.Customization) {
		return status, false, &NewGameTransactionError{
			Code:    "new_game_control_customization_mismatch",
			Message: "Control 内存中的角色身份或外观字段与新建存档配置不一致",
		}
	}
	if !status.FarmCaveChoiceApplied || !status.FarmCaveChoiceVerified || status.FarmCaveChoiceVerifiedAt == nil {
		return status, false, nil
	}
	if status.FarmCaveChoiceVerifiedAt.Before(freshAfter) || status.FarmCaveChoiceVerifiedAt.After(status.UpdatedAt) {
		return status, false, &NewGameTransactionError{
			Code:    "new_game_control_farm_cave_unconfirmed",
			Message: "Control 的农场山洞选择复核时间不属于当前 SaveLoaded 状态",
		}
	}
	if status.FarmCaveChoiceTransactionID != transactionID || status.FarmCaveChoiceSaveID != saveID {
		return status, false, &NewGameTransactionError{
			Code:    "new_game_control_farm_cave_identity_mismatch",
			Message: "Control 的农场山洞选择快照未冻结到当前事务和目标存档",
		}
	}
	if status.FarmCaveChoice == nil || !newGameFarmCaveChoiceMatches(cfg.FarmCaveChoice, *status.FarmCaveChoice) {
		return status, false, &NewGameTransactionError{
			Code:    "new_game_control_farm_cave_mismatch",
			Message: "Control 内存中的农场山洞选择与新建存档配置不一致",
		}
	}
	players, err := readNewGamePlayersDurability(dataDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return status, false, nil
		}
		return status, false, err
	}
	if players.SaveID != saveID || players.UpdatedAt.IsZero() || players.UpdatedAt.Before(*status.CustomizationVerifiedAt) {
		return status, false, nil
	}
	hosts := 0
	for _, player := range players.Players {
		if player.IsHost {
			hosts++
			if player.Name != cfg.FarmerName {
				return status, false, &NewGameTransactionError{
					Code:    "new_game_control_host_mismatch",
					Message: "players.json 的主机角色名称与冻结配置不一致",
				}
			}
		}
	}
	if hosts != 1 {
		return status, false, &NewGameTransactionError{
			Code:    "new_game_control_host_mismatch",
			Message: "players.json 未提供唯一主机角色证据",
		}
	}
	return status, true, nil
}

func readNewGamePlayersDurability(dataDir string) (newGamePlayersDurability, error) {
	var players newGamePlayersDurability
	raw, err := readRuntimeStatusFile(filepath.Join(controlDir(dataDir), "players.json"))
	if err != nil {
		return players, err
	}
	if len(raw) == 0 || len(raw) > maxNewGameControlStatusBytes {
		return players, &NewGameTransactionError{Code: "new_game_control_players_invalid", Message: "players.json 为空或超过大小限制"}
	}
	if err := json.Unmarshal(raw, &players); err != nil {
		return players, &NewGameTransactionError{Code: "new_game_control_players_invalid", Message: "players.json 不是有效 JSON", Cause: err}
	}
	return players, nil
}

func waitForNewGameControlDurability(
	ctx context.Context,
	dataDir string,
	transactionID string,
	saveID string,
	cfg registry.NewGameConfig,
	options newGameControlDurabilityWaitOptions,
) (newGameControlDurabilityStatus, error) {
	if options.Timeout <= 0 {
		options.Timeout = 5 * time.Minute
	}
	if options.PollInterval <= 0 {
		options.PollInterval = 250 * time.Millisecond
	}
	deadline := time.NewTimer(options.Timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(options.PollInterval)
	defer ticker.Stop()
	var latest newGameControlDurabilityStatus
	for {
		if err := ctx.Err(); err != nil {
			return latest, err
		}
		status, ready, err := inspectNewGameControlDurability(
			dataDir, transactionID, saveID, cfg, options.FreshAfter,
		)
		latest = status
		var txErr *NewGameTransactionError
		if errors.As(err, &txErr) && txErr.Code == "new_game_control_identity_mismatch" &&
			options.PreviousSaveID != "" && status.NewGameTransactionID == transactionID &&
			status.SaveID == options.PreviousSaveID {
			// The exact pre-/newgame baseline is allowed to age out naturally.
			// Any other SaveLoaded identity remains a terminal mismatch.
			err = nil
			ready = false
		}
		if err != nil {
			return status, err
		}
		if ready {
			return status, nil
		}
		select {
		case <-ctx.Done():
			return latest, ctx.Err()
		case <-deadline.C:
			return latest, &NewGameTransactionError{
				Code:    "new_game_control_durability_timeout",
				Message: "等待当前事务的 SaveLoaded 与角色内存复核状态超时",
			}
		case <-ticker.C:
		}
	}
}

// submitAndWaitForNewGameDurableSave publishes at most one save-now command
// with the caller-reserved ID. Existing queued/running/terminal evidence for
// that ID is observed instead of publishing a second command after recovery.
func submitAndWaitForNewGameDurableSave(
	ctx context.Context,
	dataDir string,
	commandID string,
	transactionID string,
	saveID string,
	options newGameDurableSaveOptions,
) (CommandOutcome, error) {
	if !validCommandID(commandID) || !isValidNewGameTransactionID(transactionID) || validateSaveName(saveID) != nil {
		return CommandOutcome{}, &NewGameTransactionError{
			Code:    "new_game_durable_save_identity_invalid",
			Message: "持久保存命令的 commandId、transactionId 或 saveId 无效",
		}
	}
	if options.Timeout <= 0 {
		options.Timeout = 3 * time.Minute
	}
	if options.PollInterval <= 0 {
		options.PollInterval = 250 * time.Millisecond
	}
	if options.Publish == nil {
		options.Publish = func(dataDir, commandID, transactionID, saveID string) error {
			return writePanelCommandWithID(dataDir, commandID, "save-now", map[string]string{
				"transactionId": transactionID,
				"saveId":        saveID,
			})
		}
	}
	if options.GetOutcome == nil {
		options.GetOutcome = GetCommandOutcome
	}

	deadline := time.NewTimer(options.Timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(options.PollInterval)
	defer ticker.Stop()
	submissionObserved := false
	var latest CommandOutcome
	for {
		if err := ctx.Err(); err != nil {
			return latest, err
		}
		outcome, err := options.GetOutcome(dataDir, commandID)
		latest = outcome
		if err != nil {
			return outcome, &NewGameTransactionError{
				Code:    "new_game_durable_save_result_unreadable",
				Message: "无法读取 save-now 的持久命令结果",
				Cause:   err,
			}
		}
		if outcome.CommandID != commandID {
			return outcome, &NewGameTransactionError{
				Code:    "new_game_durable_save_result_mismatch",
				Message: "save-now 返回了不同的 commandId",
			}
		}
		switch outcome.Status {
		case CommandStatusSucceeded:
			if err := validateNewGameDurableSaveOutcome(outcome, transactionID, saveID, options.FreshAfter); err != nil {
				return outcome, err
			}
			return outcome, nil
		case CommandStatusFailed:
			return outcome, &NewGameTransactionError{
				Code:    "new_game_durable_save_failed",
				Message: "save-now 在确认 GameLoop.Saved 前失败",
			}
		case CommandStatusExpired, CommandStatusDispatched:
			return outcome, &NewGameTransactionError{
				Code:    "new_game_durable_save_unconfirmed",
				Message: "save-now 结果已过期或没有提供 GameLoop.Saved 终态",
			}
		case CommandStatusQueued, CommandStatusRunning:
			submissionObserved = true
		case CommandStatusUnknown:
			if outcome.ErrorCode != "" {
				return outcome, &NewGameTransactionError{
					Code:    "new_game_durable_save_unconfirmed",
					Message: "save-now 命令结果未知，禁止自动重提",
				}
			}
			if !submissionObserved {
				if err := ctx.Err(); err != nil {
					return outcome, err
				}
				if err := options.Publish(dataDir, commandID, transactionID, saveID); err != nil {
					return outcome, &NewGameTransactionError{
						Code:    "new_game_durable_save_publish_failed",
						Message: "无法发布事务绑定的 save-now 命令",
						Cause:   err,
					}
				}
				submissionObserved = true
			}
		default:
			return outcome, &NewGameTransactionError{
				Code:    "new_game_durable_save_unconfirmed",
				Message: "save-now 命令结果状态无效",
			}
		}

		select {
		case <-ctx.Done():
			return latest, ctx.Err()
		case <-deadline.C:
			return latest, &NewGameTransactionError{
				Code:    "new_game_durable_save_timeout",
				Message: "等待同一 commandId 的 GameLoop.Saved 结果超时；禁止自动重提",
			}
		case <-ticker.C:
		}
	}
}

func validateNewGameDurableSaveOutcome(outcome CommandOutcome, transactionID, saveID string, freshAfter time.Time) error {
	if outcome.CreatedAt.IsZero() || outcome.UpdatedAt.IsZero() || outcome.UpdatedAt.Before(outcome.CreatedAt) ||
		(!freshAfter.IsZero() && outcome.CreatedAt.Before(freshAfter)) {
		return &NewGameTransactionError{
			Code:    "new_game_durable_save_result_stale",
			Message: "save-now 成功结果的时间证据早于当前 SaveLoaded 角色复核门禁",
		}
	}
	if outcome.ErrorCode != "ok" || outcome.Details == nil ||
		outcome.Details["event"] != "GameLoop.Saved" ||
		outcome.Details["transactionId"] != transactionID ||
		outcome.Details["saveId"] != saveID ||
		outcome.Details["expectedSaveId"] != saveID {
		return &NewGameTransactionError{
			Code:    "new_game_durable_save_evidence_mismatch",
			Message: "save-now 成功结果缺少当前事务、目标存档或 GameLoop.Saved 的精确证据",
		}
	}
	return nil
}

// waitForNewGameDiskDurability must run only after a validated GameLoop.Saved
// outcome. Missing, empty, or temporarily malformed XML remains pending; a
// complete XML document with a wrong identity/value is a terminal mismatch.
// Two identical valid hash pairs are required before returning success.
func waitForNewGameDiskDurability(
	ctx context.Context,
	dataDir string,
	saveID string,
	cfg registry.NewGameConfig,
	options newGameDiskDurabilityWaitOptions,
) (newGameDiskDurabilityEvidence, error) {
	if options.Timeout <= 0 {
		options.Timeout = 2 * time.Minute
	}
	if options.PollInterval <= 0 {
		options.PollInterval = 250 * time.Millisecond
	}
	deadline := time.NewTimer(options.Timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(options.PollInterval)
	defer ticker.Stop()
	var previous newGameDiskDurabilityEvidence
	stableCount := 0
	for {
		if err := ctx.Err(); err != nil {
			return previous, err
		}
		evidence, ready, err := inspectNewGameDiskDurability(
			dataDir, saveID, cfg, options.ExpectedFarmType,
		)
		if err != nil {
			return evidence, err
		}
		if ready {
			if evidence.MainSHA256 == previous.MainSHA256 &&
				evidence.SaveGameInfoSHA256 == previous.SaveGameInfoSHA256 &&
				evidence.SaveGameInfoPath == previous.SaveGameInfoPath {
				stableCount++
			} else {
				previous = evidence
				stableCount = 1
			}
			if stableCount >= 2 {
				return evidence, nil
			}
		} else {
			previous = newGameDiskDurabilityEvidence{}
			stableCount = 0
		}
		select {
		case <-ctx.Done():
			return previous, ctx.Err()
		case <-deadline.C:
			return previous, &NewGameTransactionError{
				Code:    "new_game_disk_durability_timeout",
				Message: "等待 save-now 后稳定且可解析的主存档与 SaveGameInfo 超时",
			}
		case <-ticker.C:
		}
	}
}

func inspectNewGameDiskDurability(
	dataDir string,
	saveID string,
	cfg registry.NewGameConfig,
	expectedFarmType string,
) (newGameDiskDurabilityEvidence, bool, error) {
	var evidence newGameDiskDurabilityEvidence
	if err := validateSaveName(saveID); err != nil {
		return evidence, false, &NewGameTransactionError{
			Code:    "new_game_disk_identity_invalid",
			Message: "目标存档目录名无效",
			Cause:   err,
		}
	}
	if expectedFarmType == "" {
		expectedFarmType = cfg.FarmType
	}
	expectedFarm, err := NormalizeNewGameFarmType(expectedFarmType)
	if err != nil {
		return evidence, false, &NewGameTransactionError{
			Code:    "new_game_disk_expected_farm_invalid",
			Message: "持久校验使用的目标农场类型无效",
			Cause:   err,
		}
	}
	if expectedFarm.CompatibilityID {
		return evidence, false, &NewGameTransactionError{
			Code:    "new_game_disk_expected_farm_ambiguous",
			Message: "兼容农场标识 modded 不能用于严格磁盘校验；必须传入运行时解析后的农场 ID",
		}
	}

	root := filepath.Join(savesDir(dataDir), "Saves", saveID)
	mainPath := filepath.Join(root, saveID)
	mainData, mainHash, pending, err := readNewGameDurableXMLFile(mainPath)
	if err != nil {
		return evidence, false, err
	}
	if pending {
		return evidence, false, nil
	}
	infoPath, infoData, infoHash, pending, err := readNewGameSaveGameInfo(root)
	if err != nil {
		return evidence, false, err
	}
	if pending {
		return evidence, false, nil
	}

	main, parsePending, err := parseNewGameMainSave(mainData)
	if err != nil {
		return evidence, false, err
	}
	if parsePending {
		return evidence, false, nil
	}
	info, parsePending, err := parseNewGameSaveGameInfo(infoData)
	if err != nil {
		return evidence, false, err
	}
	if parsePending {
		return evidence, false, nil
	}

	expected := expectedNewGameCoreCustomization(cfg)
	actual := newGameCoreCustomization{
		FarmerName:    main.Player.Name,
		FarmName:      main.Player.FarmName,
		FavoriteThing: main.Player.FavoriteThing,
		Gender:        genderFromSavePlayer(main.Player.Gender, main.Player.LegacyGender, main.Player.IsMale),
		PetType:       main.Player.WhichPetType,
		PetBreed:      main.Player.WhichPetBreed,
		Skin:          main.Player.Skin,
		Hair:          main.Player.Hair,
		Shirt:         newGameEffectiveClothingID(main.Player.Shirt, main.Player.ShirtItem.ItemID),
		Pants:         newGameEffectiveClothingID(main.Player.Pants, main.Player.PantsItem.ItemID),
		Accessory:     main.Player.Accessory,
		EyeColor:      newGameRGBColorFromXML(main.Player.NewEyeColor),
		HairColor:     newGameRGBColorFromXML(main.Player.HairstyleColor),
		PantsColor:    newGameRGBColorFromXML(main.Player.PantsColor),
		IsCustomized:  main.Player.IsCustomized != nil && *main.Player.IsCustomized,
	}
	if mismatches := newGameCustomizationMismatchFields(expected, actual); len(mismatches) > 0 {
		return evidence, false, &NewGameTransactionError{
			Code:    "new_game_disk_character_mismatch",
			Message: "主存档 XML 的角色身份、外观字段或 isCustomized 与冻结配置不一致（不匹配字段：" + strings.Join(mismatches, ", ") + "）",
		}
	}
	if !newGameDiskFarmCaveChoiceMatches(cfg.FarmCaveChoice, main.Player.CaveChoice, main.Player.EventsSeen) {
		return evidence, false, &NewGameTransactionError{
			Code:    "new_game_disk_farm_cave_mismatch",
			Message: "主存档 XML 的 caveChoice 或山洞选择事件状态与冻结配置不一致",
		}
	}
	actualFarm := farmTypeLabelFromString(main.WhichFarm)
	if actualFarm == "" || actualFarm != expectedFarm.ID {
		return evidence, false, &NewGameTransactionError{
			Code:    "new_game_disk_farm_type_mismatch",
			Message: fmt.Sprintf("主存档 XML 的 whichFarm 与目标农场不一致：期望 %s，实际 %s", expectedFarm.ID, actualFarm),
		}
	}
	if info.Name != cfg.FarmerName || info.FarmName != cfg.FarmName {
		return evidence, false, &NewGameTransactionError{
			Code:    "new_game_disk_save_info_mismatch",
			Message: "SaveGameInfo 的玩家名或农场名与冻结配置不一致",
		}
	}
	evidence.MainSHA256 = mainHash
	evidence.SaveGameInfoSHA256 = infoHash
	evidence.SaveGameInfoPath = filepath.Base(infoPath)
	return evidence, true, nil
}

func expectedNewGameCoreCustomization(cfg registry.NewGameConfig) newGameCoreCustomization {
	favorite := cfg.FavoriteThing
	if strings.TrimSpace(favorite) == "" {
		favorite = "Anxi"
	}
	gender := "male"
	if strings.EqualFold(cfg.Gender, "female") {
		gender = "female"
	}
	petType := "Cat"
	if strings.EqualFold(cfg.PetType, "Dog") {
		petType = "Dog"
	}
	petBreed := cfg.PetBreedID
	if strings.TrimSpace(petBreed) == "" {
		petBreed = strconv.Itoa(cfg.PetBreed)
	}
	return newGameCoreCustomization{
		FarmerName:    cfg.FarmerName,
		FarmName:      cfg.FarmName,
		FavoriteThing: favorite,
		Gender:        gender,
		PetType:       petType,
		PetBreed:      petBreed,
		Skin:          copyOptionalInt(cfg.Skin),
		Hair:          copyOptionalInt(cfg.Hair),
		Shirt:         optionalAppearanceString(cfg.Shirt),
		Pants:         optionalAppearanceString(cfg.Pants),
		Accessory:     copyOptionalInt(cfg.Accessory),
		EyeColor:      expectedNewGameRGBColor(cfg.EyeColor),
		HairColor:     expectedNewGameRGBColor(cfg.HairColor),
		PantsColor:    expectedNewGameRGBColor(cfg.PantsColor),
		IsCustomized:  true,
	}
}

func newGameFarmCaveChoiceMatches(requested string, actual newGameFarmCaveChoice) bool {
	if requested == "" {
		requested = "vanilla"
	}
	if actual.RequestedChoice != requested {
		return false
	}
	switch requested {
	case "vanilla":
		return actual.ActualChoice == 0 && !actual.ChoiceEventSeen && !actual.MushroomObjectsPresent
	case "bats":
		return actual.ActualChoice == 1 && actual.ChoiceEventSeen && !actual.MushroomObjectsPresent
	case "mushrooms":
		return actual.ActualChoice == 2 && actual.ChoiceEventSeen && actual.MushroomHouseReady
	default:
		return false
	}
}

func newGameDiskFarmCaveChoiceMatches(requested string, actual *int, eventsSeen []string) bool {
	if actual == nil {
		return false
	}
	if requested == "" {
		requested = "vanilla"
	}
	eventSeen := slices.Contains(eventsSeen, "65")
	switch requested {
	case "vanilla":
		return *actual == 0 && !eventSeen
	case "bats":
		return *actual == 1 && eventSeen
	case "mushrooms":
		return *actual == 2 && eventSeen
	default:
		return false
	}
}

func newGameCustomizationMatches(expected, actual newGameCoreCustomization) bool {
	return len(newGameCustomizationMismatchFields(expected, actual)) == 0

}

func newGameCustomizationMismatchFields(expected, actual newGameCoreCustomization) []string {
	var mismatches []string
	if expected.FarmerName != actual.FarmerName {
		mismatches = append(mismatches, "farmerName")
	}
	if expected.FarmName != actual.FarmName {
		mismatches = append(mismatches, "farmName")
	}
	if expected.FavoriteThing != actual.FavoriteThing {
		mismatches = append(mismatches, "favoriteThing")
	}
	if expected.Gender != actual.Gender {
		mismatches = append(mismatches, "gender")
	}
	if expected.PetType != actual.PetType {
		mismatches = append(mismatches, "petType")
	}
	if expected.PetBreed != actual.PetBreed {
		mismatches = append(mismatches, "petBreed")
	}
	if !optionalIntMatches(expected.Skin, actual.Skin) {
		mismatches = append(mismatches, "skin")
	}
	if !optionalIntMatches(expected.Hair, actual.Hair) {
		mismatches = append(mismatches, "hair")
	}
	if expected.Shirt != "" && expected.Shirt != actual.Shirt {
		mismatches = append(mismatches, "shirt")
	}
	if expected.Pants != "" && expected.Pants != actual.Pants {
		mismatches = append(mismatches, "pants")
	}
	if !optionalIntMatches(expected.Accessory, actual.Accessory) {
		mismatches = append(mismatches, "accessory")
	}
	if !optionalNewGameRGBMatches(expected.EyeColor, actual.EyeColor) {
		mismatches = append(mismatches, "eyeColor")
	}
	if !optionalNewGameRGBMatches(expected.HairColor, actual.HairColor) {
		mismatches = append(mismatches, "hairColor")
	}
	if !optionalNewGameRGBMatches(expected.PantsColor, actual.PantsColor) {
		mismatches = append(mismatches, "pantsColor")
	}
	if expected.IsCustomized != actual.IsCustomized {
		mismatches = append(mismatches, "isCustomized")
	}
	return mismatches
}

func copyOptionalInt(value *int) *int {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func optionalIntMatches(expected, actual *int) bool {
	return expected == nil || (actual != nil && *expected == *actual)
}

func optionalAppearanceString(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return value
}

func expectedNewGameRGBColor(color *registry.RgbColor) *newGameRGBColor {
	if color == nil {
		return nil
	}
	return &newGameRGBColor{
		R: clampNewGameColorChannel(color.R),
		G: clampNewGameColorChannel(color.G),
		B: clampNewGameColorChannel(color.B),
	}
}

func optionalNewGameRGBMatches(expected, actual *newGameRGBColor) bool {
	return expected == nil || (actual != nil && *expected == *actual)
}

func clampNewGameColorChannel(value int) int {
	if value < 0 {
		return 0
	}
	if value > 255 {
		return 255
	}
	return value
}

type newGameMainSaveXML struct {
	XMLName xml.Name `xml:"SaveGame"`
	Player  struct {
		Name           string                 `xml:"name"`
		FarmName       string                 `xml:"farmName"`
		FavoriteThing  string                 `xml:"favoriteThing"`
		Gender         string                 `xml:"Gender"`
		LegacyGender   string                 `xml:"gender"`
		IsMale         *string                `xml:"isMale"`
		WhichPetType   string                 `xml:"whichPetType"`
		WhichPetBreed  string                 `xml:"whichPetBreed"`
		Skin           *int                   `xml:"skin"`
		Hair           *int                   `xml:"hair"`
		Shirt          *string                `xml:"shirt"`
		Pants          *string                `xml:"pants"`
		ShirtItem      newGameClothingItemXML `xml:"shirtItem"`
		PantsItem      newGameClothingItemXML `xml:"pantsItem"`
		Accessory      *int                   `xml:"accessory"`
		NewEyeColor    *newGameColorXML       `xml:"newEyeColor"`
		HairstyleColor *newGameColorXML       `xml:"hairstyleColor"`
		PantsColor     *newGameColorXML       `xml:"pantsColor"`
		IsCustomized   *bool                  `xml:"isCustomized"`
		CaveChoice     *int                   `xml:"caveChoice"`
		EventsSeen     []string               `xml:"eventsSeen>int"`
	} `xml:"player"`
	WhichFarm string `xml:"whichFarm"`
}

type newGameColorXML struct {
	R *int `xml:"R"`
	G *int `xml:"G"`
	B *int `xml:"B"`
}

type newGameClothingItemXML struct {
	ItemID string `xml:"itemId"`
}

type newGameSaveGameInfoXML struct {
	XMLName  xml.Name `xml:"Farmer"`
	Name     string   `xml:"name"`
	FarmName string   `xml:"farmName"`
}

func parseNewGameMainSave(data []byte) (newGameMainSaveXML, bool, error) {
	var parsed newGameMainSaveXML
	if err := xml.Unmarshal(data, &parsed); err != nil {
		return parsed, true, nil
	}
	if parsed.XMLName.Local != "SaveGame" || genderFromSavePlayer(parsed.Player.Gender, parsed.Player.LegacyGender, parsed.Player.IsMale) == "" {
		return parsed, false, &NewGameTransactionError{
			Code:    "new_game_disk_main_schema_invalid",
			Message: "主存档 XML 缺少 SaveGame/player/有效性别结构",
		}
	}
	return parsed, false, nil
}

func parseNewGameSaveGameInfo(data []byte) (newGameSaveGameInfoXML, bool, error) {
	var parsed newGameSaveGameInfoXML
	if err := xml.Unmarshal(data, &parsed); err != nil {
		return parsed, true, nil
	}
	if parsed.XMLName.Local != "Farmer" {
		return parsed, false, &NewGameTransactionError{
			Code:    "new_game_disk_save_info_schema_invalid",
			Message: "SaveGameInfo XML 根节点不是 Farmer",
		}
	}
	return parsed, false, nil
}

func genderFromSavePlayer(gender, legacyGender string, isMale *string) string {
	for _, value := range []string{gender, legacyGender} {
		if strings.EqualFold(strings.TrimSpace(value), "female") {
			return "female"
		}
		if strings.EqualFold(strings.TrimSpace(value), "male") {
			return "male"
		}
	}
	if isMale == nil {
		return ""
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(*isMale))
	if err != nil {
		return ""
	}
	if parsed {
		return "male"
	}
	return "female"
}

func newGameXMLStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func newGameEffectiveClothingID(override *string, itemID string) string {
	value := strings.TrimSpace(newGameXMLStringValue(override))
	if value != "" && value != "-1" {
		return value
	}
	return strings.TrimSpace(itemID)
}

func newGameRGBColorFromXML(color *newGameColorXML) *newGameRGBColor {
	if color == nil || color.R == nil || color.G == nil || color.B == nil {
		return nil
	}
	return &newGameRGBColor{R: *color.R, G: *color.G, B: *color.B}
}

func readNewGameSaveGameInfo(root string) (string, []byte, string, bool, error) {
	for _, name := range []string{"SaveGameInfo", "SaveGameInfo.xml"} {
		path := filepath.Join(root, name)
		data, digest, pending, err := readNewGameDurableXMLFile(path)
		if err != nil {
			return "", nil, "", false, err
		}
		if pending {
			continue
		}
		return path, data, digest, false, nil
	}
	return "", nil, "", true, nil
}

func readNewGameDurableXMLFile(path string) ([]byte, string, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", true, nil
	}
	if err != nil {
		return nil, "", false, &NewGameTransactionError{
			Code:    "new_game_disk_file_unreadable",
			Message: "无法检查新建存档 XML 文件",
			Cause:   err,
		}
	}
	if !info.Mode().IsRegular() {
		return nil, "", false, &NewGameTransactionError{
			Code:    "new_game_disk_file_invalid",
			Message: "新建存档 XML 不是普通文件",
		}
	}
	if info.Size() <= 0 {
		return nil, "", true, nil
	}
	if info.Size() > maxSingleFileBytes {
		return nil, "", false, &NewGameTransactionError{
			Code:    "new_game_disk_file_too_large",
			Message: "新建存档 XML 超过单文件大小限制",
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", false, &NewGameTransactionError{
			Code:    "new_game_disk_file_unreadable",
			Message: "无法读取新建存档 XML 文件",
			Cause:   err,
		}
	}
	if int64(len(data)) != info.Size() || len(data) == 0 {
		return nil, "", true, nil
	}
	digest := sha256.Sum256(data)
	return data, hex.EncodeToString(digest[:]), false, nil
}
