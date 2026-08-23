package stardew_junimo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

func newGameDurabilityTestConfig() registry.NewGameConfig {
	skin := 3
	hair := 14
	accessory := 7
	return registry.NewGameConfig{
		FarmName:       "Blue Farm",
		FarmType:       "forest",
		FarmCaveChoice: "mushrooms",
		FarmerName:     "Robin",
		FavoriteThing:  "Wood",
		Gender:         "female",
		PetType:        "Dog",
		PetBreed:       3,
		PetBreedID:     "3",
		CabinLayout:    "nearby",
		CabinMode:      "recommended",
		ProfitMargin:   "100",
		MoneyMode:      "shared",
		StartingCabins: 1,
		MaxPlayers:     4,
		Skin:           &skin,
		Hair:           &hair,
		Shirt:          "1001",
		Pants:          "1002",
		Accessory:      &accessory,
		EyeColor:       &registry.RgbColor{R: 11, G: 22, B: 33},
		HairColor:      &registry.RgbColor{R: 44, G: 55, B: 66},
		PantsColor:     &registry.RgbColor{R: 77, G: 88, B: 99},
	}
}

func TestExpectedNewGameCoreCustomizationClampsColorsLikeControl(t *testing.T) {
	cfg := newGameDurabilityTestConfig()
	cfg.EyeColor = &registry.RgbColor{R: -1, G: 256, B: 12}
	cfg.HairColor = &registry.RgbColor{R: 300, G: 13, B: -2}
	cfg.PantsColor = &registry.RgbColor{R: 14, G: -3, B: 400}

	expected := expectedNewGameCoreCustomization(cfg)
	if expected.EyeColor == nil || *expected.EyeColor != (newGameRGBColor{R: 0, G: 255, B: 12}) ||
		expected.HairColor == nil || *expected.HairColor != (newGameRGBColor{R: 255, G: 13, B: 0}) ||
		expected.PantsColor == nil || *expected.PantsColor != (newGameRGBColor{R: 14, G: 0, B: 255}) {
		t.Fatalf("clamped colors = eye=%+v hair=%+v pants=%+v", expected.EyeColor, expected.HairColor, expected.PantsColor)
	}
}

func exactNewGameControlStatus(transactionID, saveID string, cfg registry.NewGameConfig, updatedAt time.Time) newGameControlDurabilityStatus {
	verifiedAt := updatedAt.Add(-time.Millisecond)
	customization := expectedNewGameCoreCustomization(cfg)
	farmCaveChoice := expectedNewGameFarmCaveChoiceForTest(cfg.FarmCaveChoice)
	return newGameControlDurabilityStatus{
		State:                       "save-loaded",
		SaveID:                      saveID,
		UpdatedAt:                   updatedAt,
		NewGameTransactionID:        transactionID,
		NewGameCreationObserved:     true,
		CustomizationApplied:        true,
		CustomizationVerified:       true,
		CustomizationTransactionID:  transactionID,
		CustomizationSaveID:         saveID,
		CustomizationVerifiedAt:     &verifiedAt,
		Customization:               &customization,
		FarmCaveChoiceApplied:       true,
		FarmCaveChoiceVerified:      true,
		FarmCaveChoiceTransactionID: transactionID,
		FarmCaveChoiceSaveID:        saveID,
		FarmCaveChoiceVerifiedAt:    &verifiedAt,
		FarmCaveChoice:              &farmCaveChoice,
	}
}

func expectedNewGameFarmCaveChoiceForTest(requested string) newGameFarmCaveChoice {
	if requested == "" {
		requested = "vanilla"
	}
	choice := 0
	eventSeen := false
	mushroomReady := false
	mushroomBoxCount := 0
	dehydratorCount := 0
	switch requested {
	case "bats":
		choice = 1
		eventSeen = true
	case "mushrooms":
		choice = 2
		eventSeen = true
		mushroomReady = true
		mushroomBoxCount = 6
		dehydratorCount = 1
	}
	return newGameFarmCaveChoice{
		RequestedChoice:        requested,
		ActualChoice:           choice,
		ChoiceEventSeen:        eventSeen,
		MushroomHouseReady:     mushroomReady,
		MushroomObjectsPresent: mushroomReady,
		MushroomBoxCount:       mushroomBoxCount,
		DehydratorCount:        dehydratorCount,
	}
}

func writeNewGameControlStatusForTest(t *testing.T, dataDir string, status newGameControlDurabilityStatus) {
	t.Helper()
	if err := writeJSONAtomic(filepath.Join(controlDir(dataDir), "status.json"), status); err != nil {
		t.Fatal(err)
	}
	if status.CustomizationVerifiedAt != nil && status.Customization != nil {
		if err := writeJSONAtomic(filepath.Join(controlDir(dataDir), "players.json"), map[string]any{
			"updatedAt": status.CustomizationVerifiedAt.Add(time.Millisecond),
			"saveId":    status.SaveID,
			"players": []map[string]any{{
				"name": status.Customization.FarmerName, "isHost": true,
			}},
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWaitForNewGameControlDurabilityRequiresExactFreshEvidence(t *testing.T) {
	dataDir := t.TempDir()
	txID := strings.Repeat("a", 32)
	saveID := "BlueFarm_123456789"
	cfg := newGameDurabilityTestConfig()
	now := time.Now().UTC()
	writeNewGameControlStatusForTest(t, dataDir, exactNewGameControlStatus(txID, saveID, cfg, now))

	status, err := waitForNewGameControlDurability(context.Background(), dataDir, txID, saveID, cfg, newGameControlDurabilityWaitOptions{
		Timeout: time.Second, PollInterval: time.Millisecond, FreshAfter: now.Add(-time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "save-loaded" || status.Customization == nil || status.Customization.FarmerName != cfg.FarmerName {
		t.Fatalf("status = %+v", status)
	}
}

func TestWaitForNewGameControlDurabilityAllowsOnlyExactPreviousSaveBaselineToAdvance(t *testing.T) {
	dataDir := t.TempDir()
	txID := strings.Repeat("9", 32)
	previousSaveID := "Existing_123"
	targetSaveID := "Fresh_456"
	cfg := newGameDurabilityTestConfig()
	freshAfter := time.Now().UTC().Add(-time.Second)
	previous := exactNewGameControlStatus(txID, previousSaveID, cfg, time.Now().UTC())
	previous.NewGameCreationObserved = false
	writeNewGameControlStatusForTest(t, dataDir, previous)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		timer := time.NewTimer(30 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			done <- ctx.Err()
			return
		case <-timer.C:
		}
		target := exactNewGameControlStatus(txID, targetSaveID, cfg, time.Now().UTC())
		if err := writeNewGameJSONAtomicForTest(ctx, filepath.Join(controlDir(dataDir), "status.json"), target); err != nil {
			done <- err
			return
		}
		players := map[string]any{
			"updatedAt": target.CustomizationVerifiedAt.Add(time.Millisecond),
			"saveId":    targetSaveID,
			"players":   []map[string]any{{"name": cfg.FarmerName, "isHost": true}},
		}
		done <- writeNewGameJSONAtomicForTest(ctx, filepath.Join(controlDir(dataDir), "players.json"), players)
	}()

	status, err := waitForNewGameControlDurability(context.Background(), dataDir, txID, targetSaveID, cfg, newGameControlDurabilityWaitOptions{
		Timeout: time.Second, PollInterval: 10 * time.Millisecond, FreshAfter: freshAfter, PreviousSaveID: previousSaveID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if writeErr := <-done; writeErr != nil {
		t.Fatal(writeErr)
	}
	if status.SaveID != targetSaveID || status.NewGameTransactionID != txID {
		t.Fatalf("accepted status = %+v", status)
	}
}

func TestInspectNewGameControlDurabilityRejectsTerminalMismatches(t *testing.T) {
	txID := strings.Repeat("b", 32)
	saveID := "BlueFarm_234567890"
	cfg := newGameDurabilityTestConfig()
	now := time.Now().UTC()
	tests := []struct {
		name string
		code string
		edit func(*newGameControlDurabilityStatus)
	}{
		{name: "transaction", code: "new_game_control_identity_mismatch", edit: func(status *newGameControlDurabilityStatus) {
			status.NewGameTransactionID = strings.Repeat("c", 32)
		}},
		{name: "save", code: "new_game_control_identity_mismatch", edit: func(status *newGameControlDurabilityStatus) {
			status.SaveID = "OtherFarm_1"
		}},
		{name: "frozen identity", code: "new_game_control_customization_identity_mismatch", edit: func(status *newGameControlDurabilityStatus) {
			status.CustomizationSaveID = "OtherFarm_1"
		}},
		{name: "memory value", code: "new_game_control_customization_mismatch", edit: func(status *newGameControlDurabilityStatus) {
			copy := *status.Customization
			copy.FavoriteThing = "Stone"
			status.Customization = &copy
		}},
		{name: "skin", code: "new_game_control_customization_mismatch", edit: func(status *newGameControlDurabilityStatus) {
			copy := *status.Customization
			wrong := 4
			copy.Skin = &wrong
			status.Customization = &copy
		}},
		{name: "hair", code: "new_game_control_customization_mismatch", edit: func(status *newGameControlDurabilityStatus) {
			copy := *status.Customization
			wrong := 15
			copy.Hair = &wrong
			status.Customization = &copy
		}},
		{name: "shirt", code: "new_game_control_customization_mismatch", edit: func(status *newGameControlDurabilityStatus) {
			copy := *status.Customization
			copy.Shirt = "1003"
			status.Customization = &copy
		}},
		{name: "pants", code: "new_game_control_customization_mismatch", edit: func(status *newGameControlDurabilityStatus) {
			copy := *status.Customization
			copy.Pants = "1004"
			status.Customization = &copy
		}},
		{name: "accessory", code: "new_game_control_customization_mismatch", edit: func(status *newGameControlDurabilityStatus) {
			copy := *status.Customization
			wrong := 8
			copy.Accessory = &wrong
			status.Customization = &copy
		}},
		{name: "eye color", code: "new_game_control_customization_mismatch", edit: func(status *newGameControlDurabilityStatus) {
			copy := *status.Customization
			copy.EyeColor = &newGameRGBColor{R: 12, G: 22, B: 33}
			status.Customization = &copy
		}},
		{name: "hair color", code: "new_game_control_customization_mismatch", edit: func(status *newGameControlDurabilityStatus) {
			copy := *status.Customization
			copy.HairColor = &newGameRGBColor{R: 44, G: 56, B: 66}
			status.Customization = &copy
		}},
		{name: "pants color", code: "new_game_control_customization_mismatch", edit: func(status *newGameControlDurabilityStatus) {
			copy := *status.Customization
			copy.PantsColor = &newGameRGBColor{R: 77, G: 88, B: 100}
			status.Customization = &copy
		}},
		{name: "is customized", code: "new_game_control_customization_mismatch", edit: func(status *newGameControlDurabilityStatus) {
			copy := *status.Customization
			copy.IsCustomized = false
			status.Customization = &copy
		}},
		{name: "farm cave frozen identity", code: "new_game_control_farm_cave_identity_mismatch", edit: func(status *newGameControlDurabilityStatus) {
			status.FarmCaveChoiceSaveID = "OtherFarm_1"
		}},
		{name: "farm cave choice", code: "new_game_control_farm_cave_mismatch", edit: func(status *newGameControlDurabilityStatus) {
			copy := *status.FarmCaveChoice
			copy.ActualChoice = 1
			status.FarmCaveChoice = &copy
		}},
		{name: "mushroom house", code: "new_game_control_farm_cave_mismatch", edit: func(status *newGameControlDurabilityStatus) {
			copy := *status.FarmCaveChoice
			copy.MushroomHouseReady = false
			status.FarmCaveChoice = &copy
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := t.TempDir()
			status := exactNewGameControlStatus(txID, saveID, cfg, now)
			tt.edit(&status)
			writeNewGameControlStatusForTest(t, dataDir, status)
			_, ready, err := inspectNewGameControlDurability(dataDir, txID, saveID, cfg, now.Add(-time.Second))
			if ready {
				t.Fatal("mismatched status was accepted")
			}
			assertNewGameDurabilityErrorCode(t, err, tt.code)
		})
	}
}

func TestInspectNewGameControlDurabilityPreservesExplicitCustomizationMismatchFields(t *testing.T) {
	txID := strings.Repeat("c", 32)
	saveID := "BlueFarm_234567891"
	cfg := newGameDurabilityTestConfig()
	now := time.Now().UTC()
	dataDir := t.TempDir()
	status := exactNewGameControlStatus(txID, saveID, cfg, now)
	status.State = "save-customization-invalid"
	status.CustomizationApplied = false
	status.CustomizationVerified = false
	status.CustomizationMismatches = []string{"shirt", "pants"}
	writeNewGameControlStatusForTest(t, dataDir, status)
	_, ready, err := inspectNewGameControlDurability(dataDir, txID, saveID, cfg, now.Add(-time.Second))
	if ready {
		t.Fatal("explicit customization mismatch was accepted")
	}
	assertNewGameDurabilityErrorCode(t, err, "new_game_control_customization_mismatch")
	if !strings.Contains(err.Error(), "shirt, pants") {
		t.Fatalf("customization mismatch diagnostics lost field names: %v", err)
	}
}

func TestInspectNewGameControlDurabilityRejectsExplicitFarmCaveFailure(t *testing.T) {
	txID := strings.Repeat("f", 32)
	saveID := "BlueFarm_234567892"
	cfg := newGameDurabilityTestConfig()
	now := time.Now().UTC()
	dataDir := t.TempDir()
	status := exactNewGameControlStatus(txID, saveID, cfg, now)
	status.State = "save-cave-choice-invalid"
	status.FarmCaveChoiceApplied = false
	status.FarmCaveChoiceVerified = false
	status.FarmCaveChoiceErrorCode = "farm_cave_choice_conflict"
	writeNewGameControlStatusForTest(t, dataDir, status)
	_, ready, err := inspectNewGameControlDurability(dataDir, txID, saveID, cfg, now.Add(-time.Second))
	if ready {
		t.Fatal("explicit farm cave failure was accepted")
	}
	assertNewGameDurabilityErrorCode(t, err, "new_game_control_farm_cave_mismatch")
	if !strings.Contains(err.Error(), "farm_cave_choice_conflict") {
		t.Fatalf("farm cave diagnostics lost the error code: %v", err)
	}
}

func TestWaitForNewGameControlDurabilityTreatsMissingStaleAndPreloadAsPending(t *testing.T) {
	txID := strings.Repeat("d", 32)
	saveID := "BlueFarm_345678901"
	cfg := newGameDurabilityTestConfig()
	freshAfter := time.Now().UTC()
	for _, setup := range []func(string){
		func(string) {},
		func(dataDir string) {
			writeNewGameControlStatusForTest(t, dataDir, exactNewGameControlStatus(txID, saveID, cfg, freshAfter.Add(-time.Minute)))
		},
		func(dataDir string) {
			status := exactNewGameControlStatus(txID, saveID, cfg, freshAfter.Add(time.Second))
			status.State = "save-creating"
			writeNewGameControlStatusForTest(t, dataDir, status)
		},
		func(dataDir string) {
			status := exactNewGameControlStatus(txID, saveID, cfg, freshAfter.Add(time.Second))
			status.NewGameCreationObserved = false
			writeNewGameControlStatusForTest(t, dataDir, status)
		},
		func(dataDir string) {
			status := exactNewGameControlStatus(txID, saveID, cfg, freshAfter.Add(time.Second))
			status.CustomizationApplied = false
			status.CustomizationVerified = false
			status.CustomizationVerifiedAt = nil
			status.Customization = nil
			writeNewGameControlStatusForTest(t, dataDir, status)
		},
	} {
		dataDir := t.TempDir()
		setup(dataDir)
		_, err := waitForNewGameControlDurability(context.Background(), dataDir, txID, saveID, cfg, newGameControlDurabilityWaitOptions{
			Timeout: 8 * time.Millisecond, PollInterval: time.Millisecond, FreshAfter: freshAfter,
		})
		assertNewGameDurabilityErrorCode(t, err, "new_game_control_durability_timeout")
	}
}

func TestSubmitAndWaitForNewGameDurableSaveUsesSameCommandIDAndExactSavedEvidence(t *testing.T) {
	dataDir := t.TempDir()
	commandID := strings.Repeat("1", 32)
	txID := strings.Repeat("2", 32)
	saveID := "BlueFarm_456789012"
	var publishes atomic.Int32
	options := newGameDurableSaveOptions{
		Timeout: 100 * time.Millisecond, PollInterval: time.Millisecond,
		Publish: func(gotDataDir, gotCommandID, gotTransactionID, gotSaveID string) error {
			publishes.Add(1)
			if gotDataDir != dataDir || gotCommandID != commandID || gotTransactionID != txID || gotSaveID != saveID {
				return fmt.Errorf("publish identity mismatch")
			}
			if err := writePanelCommandWithID(gotDataDir, gotCommandID, "save-now", map[string]string{
				"transactionId": gotTransactionID, "saveId": gotSaveID,
			}); err != nil {
				return err
			}
			createdAt := time.Now().UTC()
			return writeJSONAtomic(filepath.Join(commandResultsDir(gotDataDir), gotCommandID+".json"), CommandOutcome{
				CommandID: gotCommandID, Status: CommandStatusSucceeded, ErrorCode: "ok",
				CreatedAt: createdAt, UpdatedAt: createdAt.Add(time.Millisecond),
				Details: map[string]string{
					"event": "GameLoop.Saved", "transactionId": gotTransactionID,
					"saveId": gotSaveID, "expectedSaveId": gotSaveID,
				},
			})
		},
	}
	outcome, err := submitAndWaitForNewGameDurableSave(context.Background(), dataDir, commandID, txID, saveID, options)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.CommandID != commandID || publishes.Load() != 1 {
		t.Fatalf("outcome=%+v publishes=%d", outcome, publishes.Load())
	}

	entries, err := os.ReadDir(filepath.Join(controlDir(dataDir), "commands"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("command entries=%d err=%v", len(entries), err)
	}
	raw, err := os.ReadFile(filepath.Join(controlDir(dataDir), "commands", entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	var command struct {
		ID      string            `json:"id"`
		Payload map[string]string `json:"payload"`
	}
	if err := json.Unmarshal(raw, &command); err != nil {
		t.Fatal(err)
	}
	if command.ID != commandID || command.Payload["transactionId"] != txID || command.Payload["saveId"] != saveID {
		t.Fatalf("command = %+v", command)
	}

	if _, err := submitAndWaitForNewGameDurableSave(context.Background(), dataDir, commandID, txID, saveID, options); err != nil {
		t.Fatal(err)
	}
	if publishes.Load() != 1 {
		t.Fatalf("durable retry republished command: %d", publishes.Load())
	}
}

func TestSubmitAndWaitForNewGameDurableSaveRejectsUnboundOrUnknownResult(t *testing.T) {
	commandID := strings.Repeat("3", 32)
	txID := strings.Repeat("4", 32)
	saveID := "BlueFarm_567890123"
	tests := []struct {
		name    string
		outcome CommandOutcome
		code    string
	}{
		{
			name: "wrong event",
			outcome: CommandOutcome{CommandID: commandID, Status: CommandStatusSucceeded, ErrorCode: "ok", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC().Add(time.Second), Details: map[string]string{
				"event": "Other", "transactionId": txID, "saveId": saveID, "expectedSaveId": saveID,
			}},
			code: "new_game_durable_save_evidence_mismatch",
		},
		{
			name:    "interrupted",
			outcome: CommandOutcome{CommandID: commandID, Status: CommandStatusUnknown, ErrorCode: "execution_interrupted"},
			code:    "new_game_durable_save_unconfirmed",
		},
		{
			name:    "failed",
			outcome: CommandOutcome{CommandID: commandID, Status: CommandStatusFailed, ErrorCode: "save_target_mismatch"},
			code:    "new_game_durable_save_failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := newGameDurableSaveOptions{
				Timeout: time.Second, PollInterval: time.Millisecond,
				Publish: func(string, string, string, string) error {
					t.Fatal("terminal evidence must not publish")
					return nil
				},
				GetOutcome: func(string, string) (CommandOutcome, error) { return tt.outcome, nil },
			}
			_, err := submitAndWaitForNewGameDurableSave(context.Background(), t.TempDir(), commandID, txID, saveID, options)
			assertNewGameDurabilityErrorCode(t, err, tt.code)
		})
	}
}

func TestSubmitAndWaitForNewGameDurableSaveResumesQueuedCommandWithoutRepublishing(t *testing.T) {
	commandID := strings.Repeat("5", 32)
	txID := strings.Repeat("6", 32)
	saveID := "BlueFarm_612345678"
	var reads atomic.Int32
	options := newGameDurableSaveOptions{
		Timeout: 100 * time.Millisecond, PollInterval: time.Millisecond,
		Publish: func(string, string, string, string) error {
			t.Fatal("an existing queued command must never be republished")
			return nil
		},
		GetOutcome: func(string, string) (CommandOutcome, error) {
			if reads.Add(1) < 3 {
				return CommandOutcome{CommandID: commandID, Status: CommandStatusQueued}, nil
			}
			createdAt := time.Now().UTC()
			return CommandOutcome{
				CommandID: commandID, Status: CommandStatusSucceeded, ErrorCode: "ok",
				CreatedAt: createdAt, UpdatedAt: createdAt.Add(time.Millisecond),
				Details: map[string]string{
					"event": "GameLoop.Saved", "transactionId": txID,
					"saveId": saveID, "expectedSaveId": saveID,
				},
			}, nil
		},
	}
	if _, err := submitAndWaitForNewGameDurableSave(context.Background(), t.TempDir(), commandID, txID, saveID, options); err != nil {
		t.Fatal(err)
	}
}

func TestSubmitAndWaitForNewGameDurableSaveRejectsResultBeforeSaveLoadedGate(t *testing.T) {
	commandID := strings.Repeat("7", 32)
	txID := strings.Repeat("8", 32)
	saveID := "BlueFarm_623456789"
	createdAt := time.Now().UTC()
	options := newGameDurableSaveOptions{
		Timeout: time.Second, PollInterval: time.Millisecond, FreshAfter: createdAt.Add(time.Second),
		Publish: func(string, string, string, string) error {
			t.Fatal("a stale terminal result must not be republished")
			return nil
		},
		GetOutcome: func(string, string) (CommandOutcome, error) {
			return CommandOutcome{
				CommandID: commandID, Status: CommandStatusSucceeded, ErrorCode: "ok",
				CreatedAt: createdAt, UpdatedAt: createdAt.Add(2 * time.Second),
				Details: map[string]string{
					"event": "GameLoop.Saved", "transactionId": txID,
					"saveId": saveID, "expectedSaveId": saveID,
				},
			}, nil
		},
	}
	_, err := submitAndWaitForNewGameDurableSave(context.Background(), t.TempDir(), commandID, txID, saveID, options)
	assertNewGameDurabilityErrorCode(t, err, "new_game_durable_save_result_stale")
}

func TestWaitForNewGameDiskDurabilityRequiresStableExactMainAndSaveGameInfo(t *testing.T) {
	dataDir := t.TempDir()
	saveID := "BlueFarm_678901234"
	cfg := newGameDurabilityTestConfig()
	writeNewGameDurabilityXML(t, dataDir, saveID, validNewGameMainXML(cfg, "2"), validNewGameInfoXML(cfg), false)

	evidence, err := waitForNewGameDiskDurability(context.Background(), dataDir, saveID, cfg, newGameDiskDurabilityWaitOptions{
		Timeout: 100 * time.Millisecond, PollInterval: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence.MainSHA256) != 64 || len(evidence.SaveGameInfoSHA256) != 64 || evidence.SaveGameInfoPath != "SaveGameInfo" {
		t.Fatalf("evidence = %+v", evidence)
	}
}

func TestWaitForNewGameDiskDurabilityAcceptsStardew16GenderAndClothingItems(t *testing.T) {
	dataDir := t.TempDir()
	saveID := "BlueFarm_678901235"
	cfg := newGameDurabilityTestConfig()
	cfg.Pants = "1"
	mainXML := validNewGameMainXML(cfg, "2")
	mainXML = strings.Replace(mainXML, "<isMale>false</isMale>", `<Gender>Female</Gender><gender>Female</gender><isMale xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="true" />`, 1)
	mainXML = strings.Replace(mainXML, "<shirt>1001</shirt>", "<shirt>-1</shirt><shirtItem><itemId>1001</itemId></shirtItem>", 1)
	mainXML = strings.Replace(mainXML, "<pants>1</pants>", "<pants>-1</pants><pantsItem><itemId>1</itemId></pantsItem>", 1)
	writeNewGameDurabilityXML(t, dataDir, saveID, mainXML, validNewGameInfoXML(cfg), false)

	if _, err := waitForNewGameDiskDurability(context.Background(), dataDir, saveID, cfg, newGameDiskDurabilityWaitOptions{
		Timeout: 100 * time.Millisecond, PollInterval: time.Millisecond,
	}); err != nil {
		t.Fatal(err)
	}

	wrongDataDir := t.TempDir()
	wrongXML := strings.Replace(mainXML, "<pantsItem><itemId>1</itemId></pantsItem>", "<pantsItem><itemId>2</itemId></pantsItem>", 1)
	writeNewGameDurabilityXML(t, wrongDataDir, saveID, wrongXML, validNewGameInfoXML(cfg), false)
	_, ready, err := inspectNewGameDiskDurability(wrongDataDir, saveID, cfg, "")
	if ready {
		t.Fatal("wrong Stardew 1.6 pants item ID was accepted")
	}
	assertNewGameDurabilityErrorCode(t, err, "new_game_disk_character_mismatch")
}

func TestWaitForNewGameDiskDurabilityWaitsForTransientMalformedXML(t *testing.T) {
	dataDir := t.TempDir()
	saveID := "BlueFarm_789012345"
	cfg := newGameDurabilityTestConfig()
	writeNewGameDurabilityXML(t, dataDir, saveID, "<SaveGame><player>", "<Farmer>", true)
	root := filepath.Join(savesDir(dataDir), "Saves", saveID)
	timer := time.AfterFunc(10*time.Millisecond, func() {
		_ = os.WriteFile(filepath.Join(root, saveID), []byte(validNewGameMainXML(cfg, "2")), 0o600)
		_ = os.WriteFile(filepath.Join(root, "SaveGameInfo.xml"), []byte(validNewGameInfoXML(cfg)), 0o600)
	})
	defer timer.Stop()
	if _, err := waitForNewGameDiskDurability(context.Background(), dataDir, saveID, cfg, newGameDiskDurabilityWaitOptions{
		Timeout: 200 * time.Millisecond, PollInterval: 2 * time.Millisecond,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestInspectNewGameDiskDurabilityAcceptsEveryFarmCaveChoice(t *testing.T) {
	for _, choice := range []string{"vanilla", "bats", "mushrooms"} {
		t.Run(choice, func(t *testing.T) {
			dataDir := t.TempDir()
			saveID := "BlueFarm_789012346"
			cfg := newGameDurabilityTestConfig()
			cfg.FarmCaveChoice = choice
			writeNewGameDurabilityXML(t, dataDir, saveID, validNewGameMainXML(cfg, "2"), validNewGameInfoXML(cfg), false)
			if _, ready, err := inspectNewGameDiskDurability(dataDir, saveID, cfg, ""); err != nil || !ready {
				t.Fatalf("choice %q ready=%v err=%v", choice, ready, err)
			}
		})
	}
}

func TestInspectNewGameDiskDurabilityRejectsExactFieldMismatches(t *testing.T) {
	saveID := "BlueFarm_890123456"
	cfg := newGameDurabilityTestConfig()
	tests := []struct {
		name string
		main string
		info string
		code string
	}{
		{name: "character", main: strings.Replace(validNewGameMainXML(cfg, "2"), "<favoriteThing>Wood</favoriteThing>", "<favoriteThing>Stone</favoriteThing>", 1), info: validNewGameInfoXML(cfg), code: "new_game_disk_character_mismatch"},
		{name: "skin", main: strings.Replace(validNewGameMainXML(cfg, "2"), "<skin>3</skin>", "<skin>4</skin>", 1), info: validNewGameInfoXML(cfg), code: "new_game_disk_character_mismatch"},
		{name: "hair", main: strings.Replace(validNewGameMainXML(cfg, "2"), "<hair>14</hair>", "<hair>15</hair>", 1), info: validNewGameInfoXML(cfg), code: "new_game_disk_character_mismatch"},
		{name: "shirt", main: strings.Replace(validNewGameMainXML(cfg, "2"), "<shirt>1001</shirt>", "<shirt>1003</shirt>", 1), info: validNewGameInfoXML(cfg), code: "new_game_disk_character_mismatch"},
		{name: "pants", main: strings.Replace(validNewGameMainXML(cfg, "2"), "<pants>1002</pants>", "<pants>1004</pants>", 1), info: validNewGameInfoXML(cfg), code: "new_game_disk_character_mismatch"},
		{name: "accessory", main: strings.Replace(validNewGameMainXML(cfg, "2"), "<accessory>7</accessory>", "<accessory>8</accessory>", 1), info: validNewGameInfoXML(cfg), code: "new_game_disk_character_mismatch"},
		{name: "eye color", main: strings.Replace(validNewGameMainXML(cfg, "2"), "<newEyeColor><B>33</B><G>22</G><R>11</R></newEyeColor>", "<newEyeColor><B>33</B><G>22</G><R>12</R></newEyeColor>", 1), info: validNewGameInfoXML(cfg), code: "new_game_disk_character_mismatch"},
		{name: "hair color", main: strings.Replace(validNewGameMainXML(cfg, "2"), "<hairstyleColor><B>66</B><G>55</G><R>44</R></hairstyleColor>", "<hairstyleColor><B>66</B><G>56</G><R>44</R></hairstyleColor>", 1), info: validNewGameInfoXML(cfg), code: "new_game_disk_character_mismatch"},
		{name: "pants color", main: strings.Replace(validNewGameMainXML(cfg, "2"), "<pantsColor><B>99</B><G>88</G><R>77</R></pantsColor>", "<pantsColor><B>100</B><G>88</G><R>77</R></pantsColor>", 1), info: validNewGameInfoXML(cfg), code: "new_game_disk_character_mismatch"},
		{name: "is customized", main: strings.Replace(validNewGameMainXML(cfg, "2"), "<isCustomized>true</isCustomized>", "<isCustomized>false</isCustomized>", 1), info: validNewGameInfoXML(cfg), code: "new_game_disk_character_mismatch"},
		{name: "farm cave choice", main: strings.Replace(validNewGameMainXML(cfg, "2"), "<caveChoice>2</caveChoice>", "<caveChoice>1</caveChoice>", 1), info: validNewGameInfoXML(cfg), code: "new_game_disk_farm_cave_mismatch"},
		{name: "farm cave event", main: strings.Replace(validNewGameMainXML(cfg, "2"), "<eventsSeen><int>65</int></eventsSeen>", "<eventsSeen></eventsSeen>", 1), info: validNewGameInfoXML(cfg), code: "new_game_disk_farm_cave_mismatch"},
		{name: "farm type", main: validNewGameMainXML(cfg, "0"), info: validNewGameInfoXML(cfg), code: "new_game_disk_farm_type_mismatch"},
		{name: "save info", main: validNewGameMainXML(cfg, "2"), info: strings.Replace(validNewGameInfoXML(cfg), "<name>Robin</name>", "<name>Other</name>", 1), code: "new_game_disk_save_info_mismatch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := t.TempDir()
			writeNewGameDurabilityXML(t, dataDir, saveID, tt.main, tt.info, false)
			_, ready, err := inspectNewGameDiskDurability(dataDir, saveID, cfg, "")
			if ready {
				t.Fatal("mismatched XML was accepted")
			}
			assertNewGameDurabilityErrorCode(t, err, tt.code)
		})
	}
}

func TestWaitForNewGameDiskDurabilityMissingFilesTimesOut(t *testing.T) {
	_, err := waitForNewGameDiskDurability(context.Background(), t.TempDir(), "BlueFarm_901234567", newGameDurabilityTestConfig(), newGameDiskDurabilityWaitOptions{
		Timeout: 8 * time.Millisecond, PollInterval: time.Millisecond,
	})
	assertNewGameDurabilityErrorCode(t, err, "new_game_disk_durability_timeout")
}

func TestNewGameDurabilityWaitersHonorPreCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cfg := newGameDurabilityTestConfig()
	txID := strings.Repeat("9", 32)
	commandID := strings.Repeat("a", 32)
	saveID := "BlueFarm_934567890"

	dataDir := t.TempDir()
	writeNewGameControlStatusForTest(t, dataDir, exactNewGameControlStatus(txID, saveID, cfg, time.Now().UTC()))
	if _, err := waitForNewGameControlDurability(ctx, dataDir, txID, saveID, cfg, newGameControlDurabilityWaitOptions{Timeout: time.Second}); !errors.Is(err, context.Canceled) {
		t.Fatalf("control waiter error = %v", err)
	}

	var publishes atomic.Int32
	if _, err := submitAndWaitForNewGameDurableSave(ctx, dataDir, commandID, txID, saveID, newGameDurableSaveOptions{
		Timeout: time.Second,
		Publish: func(string, string, string, string) error {
			publishes.Add(1)
			return nil
		},
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("save waiter error = %v", err)
	}
	if publishes.Load() != 0 {
		t.Fatalf("pre-canceled save waiter published %d commands", publishes.Load())
	}

	writeNewGameDurabilityXML(t, dataDir, saveID, validNewGameMainXML(cfg, "2"), validNewGameInfoXML(cfg), false)
	if _, err := waitForNewGameDiskDurability(ctx, dataDir, saveID, cfg, newGameDiskDurabilityWaitOptions{Timeout: time.Second}); !errors.Is(err, context.Canceled) {
		t.Fatalf("disk waiter error = %v", err)
	}
}

func writeNewGameDurabilityXML(t *testing.T, dataDir, saveID, mainXML, infoXML string, xmlSuffix bool) {
	t.Helper()
	root := filepath.Join(savesDir(dataDir), "Saves", saveID)
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, saveID), []byte(mainXML), 0o600); err != nil {
		t.Fatal(err)
	}
	infoName := "SaveGameInfo"
	if xmlSuffix {
		infoName += ".xml"
	}
	if err := os.WriteFile(filepath.Join(root, infoName), []byte(infoXML), 0o600); err != nil {
		t.Fatal(err)
	}
}

func validNewGameMainXML(cfg registry.NewGameConfig, whichFarm string) string {
	isMale := "true"
	if strings.EqualFold(cfg.Gender, "female") {
		isMale = "false"
	}
	expected := expectedNewGameCoreCustomization(cfg)
	caveChoice := expectedNewGameFarmCaveChoiceForTest(cfg.FarmCaveChoice)
	eventsSeen := ""
	if caveChoice.ChoiceEventSeen {
		eventsSeen = "<int>65</int>"
	}
	return fmt.Sprintf("<SaveGame><player><name>%s</name><farmName>%s</farmName><favoriteThing>%s</favoriteThing><isMale>%s</isMale><whichPetType>%s</whichPetType><whichPetBreed>%s</whichPetBreed><skin>%d</skin><hair>%d</hair><shirt>%s</shirt><pants>%s</pants><accessory>%d</accessory><newEyeColor><B>%d</B><G>%d</G><R>%d</R></newEyeColor><hairstyleColor><B>%d</B><G>%d</G><R>%d</R></hairstyleColor><pantsColor><B>%d</B><G>%d</G><R>%d</R></pantsColor><isCustomized>true</isCustomized><caveChoice>%d</caveChoice><eventsSeen>%s</eventsSeen></player><whichFarm>%s</whichFarm></SaveGame>",
		expected.FarmerName, expected.FarmName, expected.FavoriteThing, isMale, expected.PetType, expected.PetBreed,
		newGameDurabilityIntValue(expected.Skin), newGameDurabilityIntValue(expected.Hair), expected.Shirt, expected.Pants, newGameDurabilityIntValue(expected.Accessory),
		newGameDurabilityColor(expected.EyeColor).B, newGameDurabilityColor(expected.EyeColor).G, newGameDurabilityColor(expected.EyeColor).R,
		newGameDurabilityColor(expected.HairColor).B, newGameDurabilityColor(expected.HairColor).G, newGameDurabilityColor(expected.HairColor).R,
		newGameDurabilityColor(expected.PantsColor).B, newGameDurabilityColor(expected.PantsColor).G, newGameDurabilityColor(expected.PantsColor).R,
		caveChoice.ActualChoice, eventsSeen, whichFarm)
}

func newGameDurabilityIntValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func newGameDurabilityColor(value *newGameRGBColor) newGameRGBColor {
	if value == nil {
		return newGameRGBColor{}
	}
	return *value
}

func validNewGameInfoXML(cfg registry.NewGameConfig) string {
	return fmt.Sprintf("<Farmer><name>%s</name><farmName>%s</farmName></Farmer>", cfg.FarmerName, cfg.FarmName)
}

func assertNewGameDurabilityErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s", code)
	}
	var txErr *NewGameTransactionError
	if !errors.As(err, &txErr) || txErr.Code != code {
		t.Fatalf("error = %v, want code %s", err, code)
	}
}
