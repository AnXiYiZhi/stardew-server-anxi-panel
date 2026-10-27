package updatecheck

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const BridgeVersion = "0.7.3"
const releaseMetadataStart = "<!-- anxi-panel-update\n"
const releaseMetadataEnd = "\n-->"

type Notice struct {
	Level                string `json:"level"`
	Title                string `json:"title"`
	Message              string `json:"message"`
	RequiresConfirmation bool   `json:"requiresConfirmation"`
}

// ReleaseContent travels with the immutable GitHub release, including notices
// that must be acknowledged before a new upgrade authorization is created.
type ReleaseContent struct {
	SchemaVersion         int      `json:"schemaVersion"`
	Version               string   `json:"version,omitempty"`
	MinimumUpdaterVersion string   `json:"minimumUpdaterVersion,omitempty"`
	Title                 string   `json:"title,omitempty"`
	Summary               string   `json:"summary,omitempty"`
	Changes               []string `json:"changes,omitempty"`
	Notices               []Notice `json:"notices,omitempty"`
	Blocked               bool     `json:"blocked,omitempty"`
	Notes                 string   `json:"notes,omitempty"`
}

func AtLeast(version, minimum string) bool {
	v, ok := parseSemver(version)
	m, valid := parseSemver(minimum)
	return ok && valid && compareSemver(v, m) >= 0
}

func RequiredUpdater(target string) string {
	if AtLeast(target, "0.8.0") {
		return BridgeVersion
	}
	return ""
}

func ValidateTransition(current, target string) error {
	if minimum := RequiredUpdater(target); minimum != "" && !AtLeast(current, minimum) {
		return fmt.Errorf("请先升级到 v%s，完成升级准备后再安装 v%s", minimum, strings.TrimPrefix(target, "v"))
	}
	return nil
}

func releaseContent(release githubRelease) (ReleaseContent, error) {
	release.Body = strings.ReplaceAll(release.Body, "\r\n", "\n")
	content := ReleaseContent{Title: release.Name, Notes: strings.TrimSpace(release.Body)}
	start := strings.Index(release.Body, releaseMetadataStart)
	if start >= 0 {
		raw := release.Body[start+len(releaseMetadataStart):]
		end := strings.Index(raw, releaseMetadataEnd)
		if end < 0 {
			return content, errors.New("更新说明中的升级策略不完整")
		}
		if err := json.Unmarshal([]byte(raw[:end]), &content); err != nil || content.SchemaVersion != 1 {
			return content, errors.New("更新说明中的升级策略无效")
		}
		content.Notes = strings.TrimSpace(release.Body[:start] + raw[end+len(releaseMetadataEnd):])
		if content.Version != "" && strings.TrimPrefix(content.Version, "v") != strings.TrimPrefix(release.TagName, "v") {
			return content, errors.New("更新说明与目标版本不一致")
		}
	}
	if minimum := RequiredUpdater(release.TagName); minimum != "" && !AtLeast(content.MinimumUpdaterVersion, minimum) {
		content.MinimumUpdaterVersion = minimum
	}
	if content.MinimumUpdaterVersion != "" {
		if _, ok := parseSemver(content.MinimumUpdaterVersion); !ok {
			return content, errors.New("更新前置版本无效")
		}
	}
	return content, nil
}

func noticeDigest(version string, content ReleaseContent) string {
	for _, notice := range content.Notices {
		if notice.RequiresConfirmation {
			raw, _ := json.Marshal(struct {
				Version string
				Content ReleaseContent
			}{version, content})
			digest := sha256.Sum256(raw)
			return hex.EncodeToString(digest[:])
		}
	}
	return ""
}

func (status Status) ValidateAction(target, acknowledgement string, confirm bool) error {
	if status.CheckStatus != StatusOK {
		return errors.New("请先成功检查更新，再继续升级")
	}
	if strings.TrimPrefix(strings.ToLower(target), "v") != strings.TrimPrefix(strings.ToLower(status.LatestVersion), "v") || !status.UpdateAvailable {
		return errors.New("可升级版本已变化，请重新检查更新")
	}
	if status.Content.Blocked {
		return errors.New("此版本暂缓升级，请查看重要提示")
	}
	if err := ValidateTransition(status.CurrentVersion, target); err != nil {
		return err
	}
	if minimum := status.Content.MinimumUpdaterVersion; minimum != "" && !AtLeast(status.CurrentVersion, minimum) {
		return fmt.Errorf("请先升级到 v%s", minimum)
	}
	if confirm {
		return status.ValidateNotice(acknowledgement)
	}
	return nil
}

func (status Status) ValidateNotice(acknowledgement string) error {
	if status.Content.Blocked {
		return errors.New("此版本暂缓升级，请查看重要提示")
	}
	if status.NoticeDigest != "" && status.NoticeDigest != acknowledgement {
		return errors.New("请阅读并确认当前版本的重要提示")
	}
	return nil
}
