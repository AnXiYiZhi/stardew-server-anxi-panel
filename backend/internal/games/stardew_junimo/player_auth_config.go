package stardew_junimo

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
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
	defaultAuthTimeoutSeconds   = 120
	maxAuthTimeoutSeconds       = 3600
	defaultMaxLoginAttempts     = 3
	maxLoginAttempts            = 20
	invalidRolePasswordSentinel = "\x00sap-role-auth-invalid"
)

type PlayerAuthRoleConfig struct {
	RoleID           string `json:"roleId"`
	Name             string `json:"name"`
	Configured       bool   `json:"configured"`
	CredentialStatus string `json:"credentialStatus"`
	Status           string `json:"status,omitempty"`
}

type PlayerAuthConfigResult struct {
	Mode                      string                 `json:"mode"`
	Revision                  string                 `json:"revision"`
	TimeoutSeconds            int                    `json:"timeoutSeconds"`
	MaxAttempts               int                    `json:"maxAttempts"`
	GlobalPassword            string                 `json:"globalPassword,omitempty"`
	Roles                     []PlayerAuthRoleConfig `json:"roles"`
	ConfiguredRoleCount       int                    `json:"configuredRoleCount"`
	UnconfiguredRoleCount     int                    `json:"unconfiguredRoleCount"`
	CredentialErrorCount      int                    `json:"credentialErrorCount"`
	OrphanedRoleCount         int                    `json:"orphanedRoleCount"`
	RoleCredentialStoreReady  bool                   `json:"roleCredentialStoreReady"`
	RoleCredentialStoreDetail string                 `json:"roleCredentialStoreDetail,omitempty"`
	RuntimeMode               string                 `json:"runtimeMode,omitempty"`
	RuntimeRevision           string                 `json:"runtimeRevision,omitempty"`
	RestartRequired           bool                   `json:"restartRequired"`
	RolePasswordPatchReady    bool                   `json:"rolePasswordPatchReady"`
	RolePasswordPatchDetail   string                 `json:"rolePasswordPatchDetail,omitempty"`
}

type PlayerAuthPasswordUpdate struct {
	RoleID   string `json:"roleId"`
	Password string `json:"password"`
}

type UpdatePlayerAuthConfigRequest struct {
	ExpectedRevision     string                     `json:"expectedRevision"`
	Mode                 string                     `json:"mode"`
	TimeoutSeconds       *int                       `json:"timeoutSeconds,omitempty"`
	MaxAttempts          *int                       `json:"maxAttempts,omitempty"`
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
	TimeoutSeconds int
	MaxAttempts    int
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
		timeoutSeconds := current.TimeoutSeconds
		if request.TimeoutSeconds != nil {
			if *request.TimeoutSeconds < 0 || *request.TimeoutSeconds > maxAuthTimeoutSeconds {
				return &CommandError{Code: "invalid_auth_timeout", Message: "认证超时时间必须为 0 到 3600 秒，0 表示不限制"}
			}
			timeoutSeconds = *request.TimeoutSeconds
		}
		loginAttempts := current.MaxAttempts
		if request.MaxAttempts != nil {
			if *request.MaxAttempts < 1 || *request.MaxAttempts > maxLoginAttempts {
				return &CommandError{Code: "invalid_max_login_attempts", Message: "最大失败次数必须为 1 到 20 次"}
			}
			loginAttempts = *request.MaxAttempts
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

		roleKey := current.RoleKey
		if len(roleKey) == 0 && (mode == PlayerAuthModeRole || len(request.RolePasswordUpdates) > 0) {
			existingRecords, _, storeErr := roleCredentialsForSave(instance.DataDir, players.SaveID, current.Payload)
			if storeErr != nil {
				return &CommandError{Code: "role_credential_store_invalid", Message: "角色凭据文件损坏或无法读取；为防止串号，登录和修改均已拒绝"}
			}
			if len(existingRecords) > 0 {
				return &CommandError{Code: "role_auth_key_invalid", Message: "角色凭据已经存在但认证密钥缺失，不能生成新密钥覆盖现有密码"}
			}
			roleKey, err = randomBytes(roleAuthKeyBytes)
			if err != nil {
				return fmt.Errorf("generate role authentication key: %w", err)
			}
		}

		removed := make(map[string]bool, len(request.RolePasswordRemovals))
		for _, rawID := range request.RolePasswordRemovals {
			roleID := strings.TrimSpace(rawID)
			if !validPlayerAuthRoleID(roleID) {
				return &CommandError{Code: "invalid_role_id", Message: "角色密码删除请求缺少角色 ID"}
			}
			if removed[roleID] {
				return &CommandError{Code: "duplicate_role_update", Message: "同一个角色不能重复删除密码"}
			}
			removed[roleID] = true
		}

		updated := make(map[string]bool, len(request.RolePasswordUpdates))
		credentialUpdates := make(map[string]rolePasswordRecord, len(request.RolePasswordUpdates))
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
				return &CommandError{Code: "invalid_role_password", Message: "角色密码必须为 1 到 128 个字符，不能包含控制字符，且空格不能位于首尾或连续出现"}
			}
			if len(roleKey) != roleAuthKeyBytes {
				return &CommandError{Code: "role_auth_key_invalid", Message: "角色密码密钥不可用，请重新保存角色密码配置"}
			}
			credentialUpdates[roleID] = rolePasswordRecord{
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
				return &CommandError{Code: "invalid_server_password", Message: "全服密码必须为 1 到 128 个字符，不能包含控制字符，且空格不能位于首尾或连续出现"}
			}
		case PlayerAuthModeRole:
			if len(roleKey) != roleAuthKeyBytes {
				return &CommandError{Code: "role_auth_key_invalid", Message: "角色密码密钥不可用，请重新保存角色密码配置"}
			}
			if _, err := readRoleCredentialStore(instance.DataDir); err != nil {
				return &CommandError{Code: "role_credential_store_invalid", Message: "角色凭据文件损坏或无法读取；为防止串号，登录和修改均已拒绝"}
			}
			serverPassword = deriveInternalServerPassword(roleKey)
		}

		revision := current.Revision
		envChanged := mode != current.Mode ||
			serverPassword != current.ServerPassword ||
			timeoutSeconds != current.TimeoutSeconds ||
			loginAttempts != current.MaxAttempts ||
			encodeRoleAuthKey(roleKey) != encodeRoleAuthKey(current.RoleKey)
		var envPath string
		var envUpdates map[string]string
		if envChanged {
			encodedPayload, err := encodeRolePasswordPayload(current.Payload)
			if err != nil {
				return err
			}
			revisionBytes, err := randomBytes(playerAuthRevisionBytes)
			if err != nil {
				return fmt.Errorf("generate player auth revision: %w", err)
			}
			revision = base64.RawURLEncoding.EncodeToString(revisionBytes)
			envUpdates = map[string]string{
				playerAuthModeEnvKey:     mode,
				playerAuthRevisionEnvKey: revision,
				roleAuthKeyEnvKey:        encodeRoleAuthKey(roleKey),
				rolePasswordsEnvKey:      encodedPayload,
				"SERVER_PASSWORD":        serverPassword,
				"AUTH_TIMEOUT_SECONDS":   strconv.Itoa(timeoutSeconds),
				"MAX_LOGIN_ATTEMPTS":     strconv.Itoa(loginAttempts),
			}
			envPath = instanceEnvPath(instance.DataDir)
			if _, err := os.Stat(envPath); os.IsNotExist(err) {
				for key, value := range sjconfig.EmptyEnvTemplate() {
					if _, configured := envUpdates[key]; !configured {
						envUpdates[key] = value
					}
				}
			} else if err != nil {
				return fmt.Errorf("inspect player authentication .env: %w", err)
			}
		}

		commitEnv := func() error {
			if !envChanged {
				return nil
			}
			return sjconfig.UpdateEnvFile(envPath, envUpdates)
		}
		credentialsChanged := len(removed) > 0 || len(credentialUpdates) > 0
		var stagedEnv *playerAuthEnvFileSnapshot
		if credentialsChanged && envChanged && len(current.RoleKey) == 0 && len(roleKey) == roleAuthKeyBytes {
			snapshot, snapshotErr := capturePlayerAuthEnvFile(instance.DataDir)
			if snapshotErr != nil {
				return fmt.Errorf("snapshot player authentication .env: %w", snapshotErr)
			}
			if stageErr := stagePlayerAuthRoleKey(instance.DataDir, snapshot.Exists, roleKey); stageErr != nil {
				if restoreErr := restorePlayerAuthEnvFile(instance.DataDir, snapshot); restoreErr != nil {
					return playerAuthTransactionRollbackError()
				}
				return fmt.Errorf("stage role authentication key: %w", stageErr)
			}
			stagedEnv = &snapshot
		}
		restoreStagedEnv := func(transactionErr error) error {
			if stagedEnv == nil {
				return transactionErr
			}
			if restoreErr := restorePlayerAuthEnvFile(instance.DataDir, *stagedEnv); restoreErr != nil {
				return playerAuthTransactionRollbackError()
			}
			return transactionErr
		}
		if credentialsChanged {
			if err := mutateRoleCredentialStoreAndCommit(instance.DataDir, players.SaveID, current.Payload, func(records map[string]rolePasswordRecord) error {
				for roleID := range removed {
					delete(records, roleID)
				}
				for roleID, record := range credentialUpdates {
					records[roleID] = record
				}
				return nil
			}, commitEnv); err != nil {
				return restoreStagedEnv(err)
			}
		} else if err := commitEnv(); err != nil {
			return err
		}
		next := playerAuthEnvState{
			Mode: mode, Revision: revision, ServerPassword: serverPassword,
			TimeoutSeconds: timeoutSeconds, MaxAttempts: loginAttempts,
			RoleKey: roleKey, Payload: current.Payload,
		}
		result = buildPlayerAuthConfigResult(instance, next, players)
		return nil
	})
	return result, err
}

func instanceEnvPath(dataDir string) string {
	return filepath.Join(dataDir, ".env")
}

type playerAuthEnvFileSnapshot struct {
	Raw    []byte
	Exists bool
}

func capturePlayerAuthEnvFile(dataDir string) (playerAuthEnvFileSnapshot, error) {
	raw, err := os.ReadFile(instanceEnvPath(dataDir))
	if errors.Is(err, os.ErrNotExist) {
		return playerAuthEnvFileSnapshot{}, nil
	}
	if err != nil {
		return playerAuthEnvFileSnapshot{}, err
	}
	return playerAuthEnvFileSnapshot{Raw: raw, Exists: true}, nil
}

func stagePlayerAuthRoleKey(dataDir string, envExists bool, roleKey []byte) error {
	updates := map[string]string{roleAuthKeyEnvKey: encodeRoleAuthKey(roleKey)}
	if !envExists {
		for key, value := range sjconfig.EmptyEnvTemplate() {
			updates[key] = value
		}
		updates[roleAuthKeyEnvKey] = encodeRoleAuthKey(roleKey)
	}
	return sjconfig.UpdateEnvFile(instanceEnvPath(dataDir), updates)
}

func restorePlayerAuthEnvFile(dataDir string, snapshot playerAuthEnvFileSnapshot) error {
	if snapshot.Exists {
		return writeRuntimeEnvBytesAtomic(dataDir, snapshot.Raw)
	}
	if err := os.Remove(instanceEnvPath(dataDir)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func playerAuthTransactionRollbackError() *CommandError {
	return &CommandError{
		Code:    "player_auth_transaction_rollback_failed",
		Message: "玩家加入保护配置提交失败，且自动回滚未能完成；登录已按安全模式拒绝，请检查实例文件",
	}
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
	timeoutSeconds := playerAuthEnvInt(values["AUTH_TIMEOUT_SECONDS"], defaultAuthTimeoutSeconds)
	loginAttempts := playerAuthEnvInt(values["MAX_LOGIN_ATTEMPTS"], defaultMaxLoginAttempts)
	if mode == PlayerAuthModeRole {
		if len(roleKey) != roleAuthKeyBytes {
			return playerAuthEnvState{}, &CommandError{Code: "role_auth_config_invalid", Message: "角色密码模式缺少有效密钥"}
		}
		if !secureStringEqual(values["SERVER_PASSWORD"], deriveInternalServerPassword(roleKey)) {
			return playerAuthEnvState{}, &CommandError{Code: "role_auth_guard_mismatch", Message: "角色密码内部保护口令不一致，请重新保存配置"}
		}
	}
	return playerAuthEnvState{
		Mode: mode, Revision: revision, ServerPassword: values["SERVER_PASSWORD"],
		TimeoutSeconds: timeoutSeconds, MaxAttempts: loginAttempts,
		RoleKey: roleKey, Payload: payload,
	}, nil
}

func playerAuthEnvInt(value string, defaultValue int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return defaultValue
	}
	return parsed
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
	saveID := ""
	if players != nil {
		saveID = players.SaveID
	}
	credentialRecords, _, credentialErr := roleCredentialsForSave(instance.DataDir, saveID, state.Payload)
	if credentialErr == nil && len(credentialRecords) > 0 && len(state.RoleKey) != roleAuthKeyBytes {
		credentialErr = fmt.Errorf("role credentials exist without their authentication key")
		credentialRecords = nil
	}
	configured := 0
	waiting := 0
	credentialErrors := 0
	currentIDs := make(map[string]bool, len(roles))
	for i := range roles {
		currentIDs[roles[i].RoleID] = true
		if credentialErr != nil {
			roles[i].CredentialStatus = RoleCredentialStatusError
			credentialErrors++
			continue
		}
		_, roles[i].Configured = credentialRecords[roles[i].RoleID]
		if roles[i].Configured {
			roles[i].CredentialStatus = RoleCredentialStatusConfigured
			configured++
		} else {
			roles[i].CredentialStatus = RoleCredentialStatusWaiting
			waiting++
		}
	}
	orphaned := 0
	for roleID := range credentialRecords {
		if !currentIDs[roleID] {
			orphaned++
		}
	}
	runtime := readRuntimePlayerAuthStatus(instance.DataDir)
	result := &PlayerAuthConfigResult{
		Mode: state.Mode, Revision: state.Revision,
		TimeoutSeconds: state.TimeoutSeconds, MaxAttempts: state.MaxAttempts,
		Roles:               roles,
		ConfiguredRoleCount: configured, UnconfiguredRoleCount: waiting, CredentialErrorCount: credentialErrors, OrphanedRoleCount: orphaned,
		RoleCredentialStoreReady: credentialErr == nil,
		RuntimeMode:              runtime.Mode, RuntimeRevision: runtime.Revision,
		RolePasswordPatchReady: runtime.PatchReady, RolePasswordPatchDetail: runtime.PatchDetail,
	}
	if credentialErr != nil {
		result.RoleCredentialStoreDetail = "角色凭据文件损坏或无法读取；登录已安全拒绝。"
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
		if player.IsHost || !validPlayerAuthRoleID(roleID) {
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

func validPlayerAuthRoleID(roleID string) bool {
	parsedID, err := strconv.ParseInt(roleID, 10, 64)
	return err == nil && parsedID != 0 && strconv.FormatInt(parsedID, 10) == roleID
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
		if !validPlayerAuthRoleID(roleID) || verifierErr != nil || len(verifier) != sha256.Size {
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
	if strings.HasPrefix(password, " ") || strings.HasSuffix(password, " ") || strings.Contains(password, "  ") {
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
