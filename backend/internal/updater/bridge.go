package updater

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/updatecheck"
)

const CodeUpgradePrerequisiteRequired = "upgrade_prerequisite_required"

type bridgeReceipt struct {
	Schema     int       `json:"schema"`
	Origin     string    `json:"origin"`
	Version    string    `json:"version"`
	PreparedAt time.Time `json:"preparedAt"`
}

func bridgePath(dataDir string) string { return filepath.Join(dataDir, "updater", "bridge-0.7.3.json") }

// BridgeReady rejects an existing installation that skipped the preparation
// release. A genuinely new installation records its origin before opening DB.
func BridgeReady(dataDir, version string) error {
	if !updatecheck.AtLeast(version, "0.8.0") {
		return nil
	}
	raw, err := os.ReadFile(bridgePath(dataDir))
	var receipt bridgeReceipt
	if err == nil {
		err = json.Unmarshal(raw, &receipt)
	}
	valid := err == nil && receipt.Schema == 1 && !receipt.PreparedAt.IsZero() &&
		(receipt.Origin == "bridge" && updatecheck.AtLeast(receipt.Version, updatecheck.BridgeVersion) && !updatecheck.AtLeast(receipt.Version, "0.8.0") ||
			receipt.Origin == "fresh" && updatecheck.AtLeast(receipt.Version, "0.8.0"))
	if !valid {
		return ValidationError{Code: CodeUpgradePrerequisiteRequired, Message: "此安装缺少 v0.7.3 升级准备记录，请先完成 v0.7.3 更新。"}
	}
	return nil
}

func RecordBridgeReady(dataDir, version string, fresh bool) error {
	origin := "bridge"
	if updatecheck.AtLeast(version, "0.8.0") {
		if !fresh {
			return BridgeReady(dataDir, version)
		}
		origin = "fresh"
	} else if !updatecheck.AtLeast(version, updatecheck.BridgeVersion) {
		return nil
	}
	path := bridgePath(dataDir)
	if _, err := os.Stat(path); err == nil {
		return BridgeReady(dataDir, version)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// Even an empty database must not turn a pending legacy upgrade into a new install.
	if fresh {
		if _, err := os.Stat(filepath.Join(dataDir, "updater", "apply-status.json")); !errors.Is(err, os.ErrNotExist) {
			return BridgeReady(dataDir, version)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	raw, err := json.Marshal(bridgeReceipt{Schema: 1, Origin: origin, Version: version, PreparedAt: time.Now().UTC()})
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, raw, 0600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func validateUpgradeTransition(current, target string) error {
	if err := updatecheck.ValidateTransition(current, target); err != nil {
		return ValidationError{Code: CodeUpgradePrerequisiteRequired, Message: err.Error()}
	}
	return nil
}
