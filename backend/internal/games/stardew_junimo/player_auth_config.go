package stardew_junimo

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const (
	PlayerAuthModeNone   = "none"
	PlayerAuthModeGlobal = "global"
	PlayerAuthModeRole   = "role"

	playerAuthModeEnvKey        = "SAP_PLAYER_AUTH_MODE"
	playerAuthRevisionEnvKey    = "SAP_PLAYER_AUTH_REVISION"
	roleAuthKeyEnvKey           = "SAP_ROLE_AUTH_KEY"
	rolePasswordsEnvKey         = "SAP_ROLE_PASSWORDS_B64"
	rolePasswordSchemaVersion   = 1
	roleAuthKeyBytes            = 32
	playerAuthRevisionBytes     = 16
	maxPlayerAuthPasswordRunes  = 128
	invalidRolePasswordSentinel = "\x00sap-role-auth-invalid"
)

type PlayerAuthRoleConfig struct {
	RoleID     string `json:"roleId"`
	Name       string `json:"name"`
	Configured bool   `json:"configured"`
	Status     string `json:"status,omitempty"`
}

type PlayerAuthConfigResult struct {
	Mode                    string                 `json:"mode"`
	Revision                string                 `json:"revision"`
	GlobalPassword          string                 `json:"globalPassword,omitempty"`
	Roles                   []PlayerAuthRoleConfig `json:"roles"`
	ConfiguredRoleCount     int                    `json:"configuredRoleCount"`
	UnconfiguredRoleCount   int                    `json:"unconfiguredRoleCount"`
	OrphanedRoleCount       int                    `json:"orphanedRoleCount"`
	RuntimeMode             string                 `json:"runtimeMode,omitempty"`
	RuntimeRevision         string                 `json:"runtimeRevision,omitempty"`
	RestartRequired         bool                   `json:"restartRequired"`
	RolePasswordPatchReady  bool                   `json:"rolePasswordPatchReady"`
	RolePasswordPatchDetail string                 `json:"rolePasswordPatchDetail,omitempty"`
}

type PlayerAuthPasswordUpdate struct {
	RoleID   string `json:"roleId"`
	Password string `json:"password"`
}

type UpdatePlayerAuthConfigRequest struct {
	ExpectedRevision     string                     `json:"expectedRevision"`
	Mode                 string                     `json:"mode"`
	GlobalPassword       *string                    `json:"globalPassword,omitempty"`
	RolePasswordUpdates  []PlayerAuthPasswordUpdate `json:"rolePasswordUpdates,omitempty"`
	RolePasswordRemovals []string                   `json:"rolePasswordRemovals,omitempty"`
}

type rolePasswordRecord struct {
	Name      string `json:"name,omitempty"`
	Verifier  string `json:"verifier"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type rolePasswordPayload struct {
	SchemaVersion int                           `json:"schemaVersion"`
	Roles         map[string]rolePasswordRecord `json:"roles"`
}

type playerAuthEnvState struct {
	Mode           string
	Revision       string
	ServerPassword string
	RoleKey        []byte
	Payload        rolePasswordPayload
}

type runtimePlayerAuthStatus struct {
	Mode        string
	Revision    string
	PatchReady  bool
	PatchDetail string
}

func (d *Driver) GetPlayerAuthConfig(ctx context.Context, instance registry.Instance) (*PlayerAuthConfigResult, error) {
	state, err := readPlayerAuthEnvState(instance.DataDir)
	if err != nil {
		return nil, err
	}
	players, err := d.ListPlayers(ctx, instance)
	if err != nil {
		return nil, err
	}
	return buildPlayerAuthConfigResult(instance, state, players), nil
}

func (d *Driver) UpdatePlayerAuthConfig(ctx context.Context, instance registry.Instance, request UpdatePlayerAuthConfigRequest) (*PlayerAuthConfigResult, error) {
	var result *PlayerAuthConfigResult
	err := d.WithMutationOwnership(ctx, instance, func() error {
		current, err := readPlayerAuthEnvState(instance.DataDir)
		if err != nil {
			return err
		}
		if strings.TrimSpace(request.ExpectedRevision) == "" || request.ExpectedRevision != current.Revision {
			return &CommandError{Code: "player_auth_revision_conflict", Message: "玩家加入保护配置已被其他操作修改，请刷新后重试"}
		}
		mode, err := normalizePlayerAuthMode(request.Mode)
		if err != nil {
			return err
		}
		players, err := d.listPlayers(ctx, instance)
		if err != nil {
			return err
		}
		roles := eligiblePlayerAuthRoles(players)
		roleByID := make(map[string]PlayerAuthRoleConfig, len(roles))
		for _, role := range roles {
			roleByID[role.RoleID] = role
		}

		payload := current.Payload
		if payload.Roles == nil {
			payload = rolePasswordPayload{SchemaVersion: rolePasswordSchemaVersion, Roles: map[string]rolePasswordRecord{}}
		}
		roleKey := current.RoleKey
		if len(roleKey) == 0 && (mode == PlayerAuthModeRole || len(request.RolePasswordUpdates) > 0) {
			roleKey, err = randomBytes(roleAuthKeyBytes)
			if err != nil {
				return fmt.Errorf("generate role authentication key: %w", err)
			}
		}

		removed := make(map[string]bool, len(request.RolePasswordRemovals))
		for _, rawID := range request.RolePasswordRemovals {
			roleID := strings.TrimSpace(rawID)
			if roleID == "" {
				return &CommandError{Code: "invalid_role_id", Message: "角色密码删除请求缺少角色 ID"}
			}
			if removed[roleID] {
				return &CommandError{Code: "duplicate_role_update", Message: "同一个角色不能重复删除密码"}
			}
			removed[roleID] = true
			delete(payload.Roles, roleID)
		}

		updated := make(map[string]bool, len(request.RolePasswordUpdates))
		for _, update := range request.RolePasswordUpdates {
			roleID := strings.TrimSpace(update.RoleID)
			role, exists := roleByID[roleID]
			if !exists {
				return &CommandError{Code: "role_not_found", Message: "找不到要设置密码的当前存档角色"}
			}
			if removed[roleID] || updated[roleID] {
				return &CommandError{Code: "duplicate_role_update", Message: "同一个角色不能同时或重复修改密码"}
			}
			if !validPlayerAuthPassword(update.Password) {
				return &CommandError{Code: "invalid_role_password", Message: "角色密码必须为 1 到 128 个字符，且不能包含控制字符"}
			}
			if len(roleKey) != roleAuthKeyBytes {
				return &CommandError{Code: "role_auth_key_invalid", Message: "角色密码密钥不可用，请重新保存角色密码配置"}
			}
			payload.Roles[roleID] = rolePasswordRecord{
				Name:      role.Name,
				Verifier:  computeRolePasswordVerifier(roleKey, roleID, update.Password),
				UpdatedAt: time.Now().UTC().Format(time.RFC3339),
			}
			updated[roleID] = true
		}

		serverPassword := current.ServerPassword
		switch mode {
		case PlayerAuthModeNone:
			serverPassword = ""
		case PlayerAuthModeGlobal:
			if request.GlobalPassword != nil {
				serverPassword = *request.GlobalPassword
			} else if current.Mode != PlayerAuthModeGlobal {
				return &CommandError{Code: "global_password_required", Message: "切换到全服统一密码时必须设置新密码"}
			}
			if !validPlayerAuthPassword(serverPassword) {
				return &CommandError{Code: "invalid_server_password", Message: "全服密码必须为 1 到 128 个字符，且不能包含控制字符"}
			}
		case PlayerAuthModeRole:
			if len(roles) == 0 {
				return &CommandError{Code: "role_roster_empty", Message: "当前存档没有可配置的非主机角色，无法启用角色独立密码"}
			}
			if len(roleKey) != roleAuthKeyBytes {
				return &CommandError{Code: "role_auth_key_invalid", Message: "角色密码密钥不可用，请重新保存角色密码配置"}
			}
			for _, role := range roles {
				if _, configured := payload.Roles[role.RoleID]; !configured {
					return &CommandError{Code: "role_passwords_incomplete", Message: "启用角色独立密码前，必须为所有当前角色设置密码"}
				}
			}
			serverPassword = deriveInternalServerPassword(roleKey)
		}

		encodedPayload, err := encodeRolePasswordPayload(payload)
		if err != nil {
			return err
		}
		revisionBytes, err := randomBytes(playerAuthRevisionBytes)
		if err != nil {
			return fmt.Errorf("generate player auth revision: %w", err)
		}
		revision := base64.RawURLEncoding.EncodeToString(revisionBytes)
		updates := map[string]string{
			playerAuthModeEnvKey:     mode,
			playerAuthRevisionEnvKey: revision,
			roleAuthKeyEnvKey:        encodeRoleAuthKey(roleKey),
			rolePasswordsEnvKey:      encodedPayload,
			"SERVER_PASSWORD":        serverPassword,
		}
		envPath := instanceEnvPath(instance.DataDir)
		if _, err := os.Stat(envPath); os.IsNotExist(err) {
			for key, value := range sjconfig.EmptyEnvTemplate() {
				if _, configured := updates[key]; !configured {
					updates[key] = value
				}
			}
		} else if err != nil {
			return fmt.Errorf("inspect player authentication .env: %w", err)
		}
		if err := sjconfig.UpdateEnvFile(envPath, updates); err != nil {
			return err
		}
		next := playerAuthEnvState{
			Mode: mode, Revision: revision, ServerPassword: serverPassword, RoleKey: roleKey, Payload: payload,
		}
		result = buildPlayerAuthConfigResult(instance, next, players)
		return nil
	})
	return result, err
}

func instanceEnvPath(dataDir string) string {
	return filepath.Join(dataDir, ".env")
}

func readPlayerAuthEnvState(dataDir string) (playerAuthEnvState, error) {
	values, err := sjconfig.ReadEnvFile(instanceEnvPath(dataDir))
	if err != nil {
		return playerAuthEnvState{}, err
	}
	mode := strings.ToLower(strings.TrimSpace(values[playerAuthModeEnvKey]))
	if mode == "" {
		if values["SERVER_PASSWORD"] == "" {
			mode = PlayerAuthModeNone
		} else {
			mode = PlayerAuthModeGlobal
		}
	}
	mode, err = normalizePlayerAuthMode(mode)
	if err != nil {
		return playerAuthEnvState{}, err
	}
	revision := strings.TrimSpace(values[playerAuthRevisionEnvKey])
	if revision == "" {
		revision = "legacy-" + mode
	}
	roleKey, err := decodeRoleAuthKey(values[roleAuthKeyEnvKey])
	if err != nil {
		return playerAuthEnvState{}, &CommandError{Code: "role_auth_config_invalid", Message: "角色密码密钥格式无效"}
	}
	payload, err := decodeRolePasswordPayload(values[rolePasswordsEnvKey])
	if err != nil {
		return playerAuthEnvState{}, &CommandError{Code: "role_auth_config_invalid", Message: "角色密码配置格式无效"}
	}
	if mode == PlayerAuthModeRole {
		if len(roleKey) != roleAuthKeyBytes || len(payload.Roles) == 0 {
			return playerAuthEnvState{}, &CommandError{Code: "role_auth_config_invalid", Message: "角色密码模式缺少有效密钥或角色配置"}
		}
		if !secureStringEqual(values["SERVER_PASSWORD"], deriveInternalServerPassword(roleKey)) {
			return playerAuthEnvState{}, &CommandError{Code: "role_auth_guard_mismatch", Message: "角色密码内部保护口令不一致，请重新保存配置"}
		}
	}
	return playerAuthEnvState{
		Mode: mode, Revision: revision, ServerPassword: values["SERVER_PASSWORD"], RoleKey: roleKey, Payload: payload,
	}, nil
}

func normalizePlayerAuthMode(mode string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case PlayerAuthModeNone:
		return PlayerAuthModeNone, nil
	case PlayerAuthModeGlobal:
		return PlayerAuthModeGlobal, nil
	case PlayerAuthModeRole:
		return PlayerAuthModeRole, nil
	default:
		return "", &CommandError{Code: "invalid_player_auth_mode", Message: "玩家加入保护模式无效"}
	}
}

func buildPlayerAuthConfigResult(instance registry.Instance, state playerAuthEnvState, players *PlayersResult) *PlayerAuthConfigResult {
	roles := eligiblePlayerAuthRoles(players)
	configured := 0
	currentIDs := make(map[string]bool, len(roles))
	for i := range roles {
		currentIDs[roles[i].RoleID] = true
		_, roles[i].Configured = state.Payload.Roles[roles[i].RoleID]
		if roles[i].Configured {
			configured++
		}
	}
	orphaned := 0
	for roleID := range state.Payload.Roles {
		if !currentIDs[roleID] {
			orphaned++
		}
	}
	runtime := readRuntimePlayerAuthStatus(instance.DataDir)
	result := &PlayerAuthConfigResult{
		Mode: state.Mode, Revision: state.Revision, Roles: roles,
		ConfiguredRoleCount: configured, UnconfiguredRoleCount: len(roles) - configured, OrphanedRoleCount: orphaned,
		RuntimeMode: runtime.Mode, RuntimeRevision: runtime.Revision,
		RolePasswordPatchReady: runtime.PatchReady, RolePasswordPatchDetail: runtime.PatchDetail,
	}
	if state.Mode == PlayerAuthModeGlobal {
		result.GlobalPassword = state.ServerPassword
	}
	if instance.State == storage.InstanceStateRunning && state.Revision != "" {
		result.RestartRequired = runtime.Revision == "" || runtime.Revision != state.Revision || (runtime.Mode != "" && runtime.Mode != state.Mode)
	}
	return result
}

func eligiblePlayerAuthRoles(players *PlayersResult) []PlayerAuthRoleConfig {
	if players == nil {
		return []PlayerAuthRoleConfig{}
	}
	byID := make(map[string]PlayerAuthRoleConfig)
	for _, player := range players.Players {
		roleID := strings.TrimSpace(player.UniqueMultiplayerID)
		parsedID, err := strconv.ParseInt(roleID, 10, 64)
		if player.IsHost || err != nil || parsedID <= 0 {
			continue
		}
		name := strings.TrimSpace(player.Name)
		if name == "" {
			name = "未命名角色"
		}
		byID[roleID] = PlayerAuthRoleConfig{RoleID: roleID, Name: name, Status: player.Status}
	}
	roles := make([]PlayerAuthRoleConfig, 0, len(byID))
	for _, role := range byID {
		roles = append(roles, role)
	}
	sort.Slice(roles, func(i, j int) bool {
		if roles[i].Name == roles[j].Name {
			return roles[i].RoleID < roles[j].RoleID
		}
		return roles[i].Name < roles[j].Name
	})
	return roles
}

func computeRolePasswordVerifier(key []byte, roleID, password string) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("sap-role-password-v1\x00"))
	_, _ = mac.Write([]byte(roleID))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(password))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func verifyRolePassword(key []byte, roleID, password, expected string) bool {
	actual, err := base64.RawURLEncoding.DecodeString(computeRolePasswordVerifier(key, roleID, password))
	if err != nil {
		return false
	}
	want, err := base64.RawURLEncoding.DecodeString(expected)
	if err != nil || len(actual) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare(actual, want) == 1
}

func deriveInternalServerPassword(key []byte) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("sap-junimo-internal-password-v1"))
	return "sap_" + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func encodeRolePasswordPayload(payload rolePasswordPayload) (string, error) {
	if payload.Roles == nil {
		payload.Roles = map[string]rolePasswordRecord{}
	}
	payload.SchemaVersion = rolePasswordSchemaVersion
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode role password config: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeRolePasswordPayload(encoded string) (rolePasswordPayload, error) {
	if strings.TrimSpace(encoded) == "" {
		return rolePasswordPayload{SchemaVersion: rolePasswordSchemaVersion, Roles: map[string]rolePasswordRecord{}}, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return rolePasswordPayload{}, err
	}
	var payload rolePasswordPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return rolePasswordPayload{}, err
	}
	if payload.SchemaVersion != rolePasswordSchemaVersion || payload.Roles == nil {
		return rolePasswordPayload{}, fmt.Errorf("unsupported role password schema")
	}
	for roleID, record := range payload.Roles {
		verifier, verifierErr := base64.RawURLEncoding.DecodeString(strings.TrimSpace(record.Verifier))
		if parsed, err := strconv.ParseInt(roleID, 10, 64); err != nil || parsed <= 0 || verifierErr != nil || len(verifier) != sha256.Size {
			return rolePasswordPayload{}, fmt.Errorf("invalid role password record")
		}
	}
	return payload, nil
}

func encodeRoleAuthKey(key []byte) string {
	if len(key) == 0 {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(key)
}

func decodeRoleAuthKey(encoded string) ([]byte, error) {
	if strings.TrimSpace(encoded) == "" {
		return nil, nil
	}
	key, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil || len(key) != roleAuthKeyBytes {
		return nil, fmt.Errorf("invalid role auth key")
	}
	return key, nil
}

func randomBytes(length int) ([]byte, error) {
	value := make([]byte, length)
	if _, err := rand.Read(value); err != nil {
		return nil, err
	}
	return value, nil
}

func secureStringEqual(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func validPlayerAuthPassword(password string) bool {
	runeCount := utf8.RuneCountInString(password)
	if runeCount == 0 || runeCount > maxPlayerAuthPasswordRunes {
		return false
	}
	for _, value := range password {
		if unicode.IsControl(value) {
			return false
		}
	}
	return true
}

func readRuntimePlayerAuthStatus(dataDir string) runtimePlayerAuthStatus {
	raw, err := os.ReadFile(filepath.Join(controlDir(dataDir), "status.json"))
	if err != nil {
		return runtimePlayerAuthStatus{}
	}
	var status struct {
		Mode        string `json:"playerAuthMode"`
		Revision    string `json:"playerAuthConfigRevision"`
		PatchReady  bool   `json:"rolePasswordPatchAvailable"`
		PatchDetail string `json:"rolePasswordPatchDetail"`
	}
	if err := json.Unmarshal(raw, &status); err != nil {
		return runtimePlayerAuthStatus{}
	}
	return runtimePlayerAuthStatus{
		Mode: strings.TrimSpace(status.Mode), Revision: strings.TrimSpace(status.Revision),
		PatchReady: status.PatchReady, PatchDetail: status.PatchDetail,
	}
}
