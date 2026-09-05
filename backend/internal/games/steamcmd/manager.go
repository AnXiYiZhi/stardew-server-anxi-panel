package steamcmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const credentialsSchemaVersion = 1

var namespacePattern = regexp.MustCompile(`[^a-z0-9_.-]+`)

// Credentials are Panel-wide Steam download credentials. They are shared by
// game installers only; instance-level lobby/invite sessions remain separate.
type Credentials struct {
	Username               string
	Password               string
	AuthorizationCompleted bool
}

type credentialsFile struct {
	SchemaVersion          int    `json:"schemaVersion"`
	Username               string `json:"username"`
	Password               string `json:"password"`
	AuthorizationCompleted bool   `json:"authorizationCompleted"`
	UpdatedAt              string `json:"updatedAt"`
}

// Manager owns the shared SteamCMD authorization volumes, credentials and a
// process-wide download gate. One Manager should be injected into every game
// driver registered by the same Panel process.
type Manager struct {
	root      string
	namespace string
	mu        sync.Mutex
	gate      chan struct{}
}

func NewManager(dataDir, namespace string) *Manager {
	namespace = strings.ToLower(strings.TrimSpace(namespace))
	namespace = strings.Trim(namespacePattern.ReplaceAllString(namespace, "-"), "-._")
	if namespace == "" {
		namespace = "anxi-panel"
	}
	manager := &Manager{
		root:      filepath.Join(filepath.Clean(dataDir), "shared", "steam-download"),
		namespace: namespace,
		gate:      make(chan struct{}, 1),
	}
	manager.gate <- struct{}{}
	return manager
}

func (m *Manager) CredentialsPath() string {
	return filepath.Join(m.root, "credentials.json")
}

func (m *Manager) AuthorizationVolumeNames() (login, home string) {
	return m.namespace + "_steamcmd-login", m.namespace + "_steamcmd-home"
}

func (m *Manager) Load() (Credentials, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.loadLocked()
}

func (m *Manager) loadLocked() (Credentials, bool, error) {
	raw, err := os.ReadFile(m.CredentialsPath())
	if errors.Is(err, os.ErrNotExist) {
		return Credentials{}, false, nil
	}
	if err != nil {
		return Credentials{}, false, fmt.Errorf("read shared Steam credentials: %w", err)
	}
	var stored credentialsFile
	if err := json.Unmarshal(raw, &stored); err != nil {
		return Credentials{}, false, fmt.Errorf("decode shared Steam credentials: %w", err)
	}
	if stored.SchemaVersion != credentialsSchemaVersion {
		return Credentials{}, false, fmt.Errorf("unsupported shared Steam credentials schema %d", stored.SchemaVersion)
	}
	credentials := Credentials{
		Username:               strings.TrimSpace(stored.Username),
		Password:               stored.Password,
		AuthorizationCompleted: stored.AuthorizationCompleted,
	}
	if credentials.Username == "" || credentials.Password == "" {
		return Credentials{}, false, nil
	}
	return credentials, true, nil
}

func (m *Manager) Save(credentials Credentials) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	credentials.Username = strings.TrimSpace(credentials.Username)
	if credentials.Username == "" || credentials.Password == "" {
		return errors.New("Steam username and password are required")
	}
	return m.writeLocked(credentials)
}

// SaveIfMissing migrates one legacy instance credential pair without
// overwriting an already configured Panel-wide account.
func (m *Manager) SaveIfMissing(credentials Credentials) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, found, err := m.loadLocked(); err != nil || found {
		return false, err
	}
	credentials.Username = strings.TrimSpace(credentials.Username)
	if credentials.Username == "" || credentials.Password == "" {
		return false, nil
	}
	if err := m.writeLocked(credentials); err != nil {
		return false, err
	}
	return true, nil
}

func (m *Manager) SetAuthorizationCompleted(completed bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	credentials, found, err := m.loadLocked()
	if err != nil {
		return err
	}
	if !found {
		return errors.New("shared Steam credentials are not configured")
	}
	credentials.AuthorizationCompleted = completed
	return m.writeLocked(credentials)
}

func (m *Manager) writeLocked(credentials Credentials) error {
	if err := os.MkdirAll(m.root, 0o700); err != nil {
		return fmt.Errorf("create shared Steam credential directory: %w", err)
	}
	payload, err := json.MarshalIndent(credentialsFile{
		SchemaVersion:          credentialsSchemaVersion,
		Username:               credentials.Username,
		Password:               credentials.Password,
		AuthorizationCompleted: credentials.AuthorizationCompleted,
		UpdatedAt:              time.Now().UTC().Format(time.RFC3339),
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode shared Steam credentials: %w", err)
	}
	payload = append(payload, '\n')
	path := m.CredentialsPath()
	file, err := os.CreateTemp(m.root, ".credentials-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary shared Steam credentials: %w", err)
	}
	temporaryPath := file.Name()
	defer os.Remove(temporaryPath)
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return fmt.Errorf("protect shared Steam credentials: %w", err)
	}
	if _, err := file.Write(payload); err != nil {
		_ = file.Close()
		return fmt.Errorf("write shared Steam credentials: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync shared Steam credentials: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close shared Steam credentials: %w", err)
	}
	if err := replaceCredentialsFile(temporaryPath, path); err != nil {
		return fmt.Errorf("publish shared Steam credentials: %w", err)
	}
	return nil
}

// AcquireDownload serializes all SteamCMD users sharing this Manager. SteamCMD
// mutates its machine authorization cache, so concurrent game downloads must
// not write that volume at the same time.
func (m *Manager) AcquireDownload(ctx context.Context) (func(), error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-m.gate:
		var once sync.Once
		return func() { once.Do(func() { m.gate <- struct{}{} }) }, nil
	}
}
