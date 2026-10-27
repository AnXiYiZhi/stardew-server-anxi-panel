package stardew_junimo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

var junimo140GroupApplyID = regexp.MustCompile(`^apply_[a-f0-9]{24}$`)
var junimo140GroupSHA256 = regexp.MustCompile(`^[a-f0-9]{64}$`)

type junimo140GroupMember struct {
	ID      string `json:"id"`
	DataDir string `json:"dataDir"`
	Project string `json:"project"`
	Archive string `json:"archive"`
}

// The private plan contains no expanded Compose environment or credentials.
// Its paths must be rebuilt against the current instance store on recovery.
type junimo140GroupPlan struct {
	SchemaVersion int                    `json:"schemaVersion"`
	ApplyID       string                 `json:"applyId"`
	PrimaryID     string                 `json:"primaryId"`
	SourceVolume  string                 `json:"sourceVolume"`
	StagingVolume string                 `json:"stagingVolume"`
	UID           string                 `json:"uid"`
	GID           string                 `json:"gid"`
	Members       []junimo140GroupMember `json:"members"`
}

func sameJunimoGroupPath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func plainJunimoGroupRoot(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || filepath.Dir(path) == path || strings.ContainsAny(path, ",\r\n\x00") {
		return errors.New("shared runtime instance path is unsafe")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || !sameJunimoGroupPath(resolved, path) {
		return errors.New("shared runtime instance path is not a plain canonical directory")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("shared runtime instance directory is unavailable")
	}
	return nil
}

func junimoGroupPathContains(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// Build from the complete authoritative instance list, not a browser-supplied
// member set. A malformed candidate or overlapping root fails before leases,
// container stops, backups or game-volume writes.
func buildJunimo140GroupPlan(instances []storage.Instance, primary registry.Instance, applyID string) (junimo140GroupPlan, error) {
	return discoverJunimo140GroupPlan(instances, primary, applyID, "", "")
}

func discoverJunimo140GroupPlan(instances []storage.Instance, primary registry.Instance, applyID, recoverySource, recoveryStaging string) (junimo140GroupPlan, error) {
	plan := junimo140GroupPlan{SchemaVersion: 1, ApplyID: applyID, PrimaryID: primary.ID}
	if !junimo140GroupApplyID.MatchString(applyID) || primary.DriverID != DriverID {
		return plan, errors.New("invalid shared runtime transaction identity")
	}
	source, err := GameDataVolumeName(primary.DataDir)
	if err != nil {
		return plan, err
	}
	if recoverySource != "" {
		if !gameDataVolumeNamePattern.MatchString(recoverySource) || !gameDataVolumeNamePattern.MatchString(recoveryStaging) {
			return plan, errors.New("invalid shared runtime recovery volumes")
		}
		source = recoverySource
	}
	plan.SourceVolume = source
	ids, projects, archives := map[string]bool{}, map[string]bool{}, map[string]bool{}
	var allRoots []string
	foundPrimary := false
	for _, instance := range instances {
		if instance.DriverID != DriverID && instance.DataDir == "" {
			continue
		}
		if err := plainJunimoGroupRoot(instance.DataDir); err != nil {
			return plan, err
		}
		for _, root := range allRoots {
			if junimoGroupPathContains(root, instance.DataDir) || junimoGroupPathContains(instance.DataDir, root) {
				return plan, errors.New("overlapping runtime instance directories require manual recovery")
			}
		}
		allRoots = append(allRoots, instance.DataDir)
	}
	for _, instance := range instances {
		if instance.DriverID != DriverID {
			continue
		}
		if ids[instance.ID] || instance.ID == "" {
			return plan, errors.New("duplicate or missing shared runtime instance identity")
		}
		ids[instance.ID] = true
		if err := plainJunimoGroupRoot(instance.DataDir); err != nil {
			return plan, err
		}
		if instance.ID == primary.ID {
			if !sameJunimoGroupPath(instance.DataDir, primary.DataDir) {
				return plan, errors.New("primary instance changed during group discovery")
			}
			foundPrimary = true
		}
		envPath := filepath.Join(instance.DataDir, ".env")
		info, err := os.Lstat(envPath)
		if errors.Is(err, os.ErrNotExist) && instance.ID != primary.ID {
			continue
		}
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return plan, errors.New("shared runtime environment is unavailable or unsafe")
		}
		volume, err := GameDataVolumeName(instance.DataDir)
		if err != nil {
			return plan, err
		}
		if volume != source && volume != recoveryStaging {
			continue
		}
		project := strings.ToLower(filepath.Base(instance.DataDir))
		if !runtimeComposeProjectPattern.MatchString(project) || projects[project] {
			return plan, errors.New("shared runtime compose project is ambiguous")
		}
		projects[project] = true
		env, err := sjconfig.ReadEnvFile(envPath)
		if err != nil {
			return plan, err
		}
		identity := func(key string) (string, error) {
			value := strings.TrimSpace(env[key])
			if value == "" {
				return "1000", nil
			}
			number, err := strconv.ParseUint(value, 10, 32)
			if err != nil || number == 4294967295 {
				return "", errors.New("shared runtime UID/GID is invalid")
			}
			return strconv.FormatUint(number, 10), nil
		}
		uid, err := identity("USER_ID")
		if err != nil {
			return plan, err
		}
		gid, err := identity("GROUP_ID")
		if err != nil {
			return plan, err
		}
		if len(plan.Members) == 0 {
			plan.UID, plan.GID = uid, gid
		} else if uid != plan.UID || gid != plan.GID {
			return plan, errors.New("shared game volume has conflicting runtime UID/GID")
		}
		hash := sha256.Sum256([]byte(instance.ID))
		archive := hex.EncodeToString(hash[:6]) + ".tar"
		if archives[archive] {
			return plan, errors.New("shared runtime archive identity collision")
		}
		archives[archive] = true
		plan.Members = append(plan.Members, junimo140GroupMember{ID: instance.ID, DataDir: instance.DataDir, Project: project, Archive: archive})
	}
	if !foundPrimary || len(plan.Members) == 0 || len(plan.Members) > 256 {
		return plan, errors.New("shared runtime group is missing or exceeds the supported bound")
	}
	sort.Slice(plan.Members, func(i, j int) bool { return plan.Members[i].ID < plan.Members[j].ID })
	for _, member := range plan.Members {
		if member.ID == primary.ID {
			plan.StagingVolume = member.Project + "_anxi-smapi-update-" + strings.TrimPrefix(applyID, "apply_")
		}
	}
	if plan.StagingVolume == "" {
		return plan, errors.New("primary instance is not in its shared game group")
	}
	return plan, nil
}

func (plan junimo140GroupPlan) primary() (junimo140GroupMember, error) {
	for _, member := range plan.Members {
		if member.ID == plan.PrimaryID {
			return member, nil
		}
	}
	return junimo140GroupMember{}, errors.New("shared runtime primary instance is missing")
}

func (plan junimo140GroupPlan) digest() string {
	raw, _ := json.Marshal(plan)
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}
