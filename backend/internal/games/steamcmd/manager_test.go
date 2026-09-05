package steamcmd

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"
)

func TestManagerPersistsCredentialsAndAuthorizationState(t *testing.T) {
	manager := NewManager(t.TempDir(), "Anxi Panel QA")
	if err := manager.Save(Credentials{Username: " steam-user ", Password: "secret"}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}
	credentials, found, err := manager.Load()
	if err != nil || !found {
		t.Fatalf("load credentials: found=%v err=%v", found, err)
	}
	if credentials.Username != "steam-user" || credentials.Password != "secret" || credentials.AuthorizationCompleted {
		t.Fatalf("credentials = %#v", credentials)
	}
	if err := manager.SetAuthorizationCompleted(true); err != nil {
		t.Fatalf("mark authorization: %v", err)
	}
	credentials, found, err = manager.Load()
	if err != nil || !found || !credentials.AuthorizationCompleted {
		t.Fatalf("authorized credentials: found=%v value=%#v err=%v", found, credentials, err)
	}
	info, err := os.Stat(manager.CredentialsPath())
	if err != nil {
		t.Fatalf("stat credentials: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("credentials mode = %o", info.Mode().Perm())
	}
	login, home := manager.AuthorizationVolumeNames()
	if login != "anxi-panel-qa_steamcmd-login" || home != "anxi-panel-qa_steamcmd-home" {
		t.Fatalf("authorization volumes = %q, %q", login, home)
	}
}

func TestManagerMigrationDoesNotOverwriteConfiguredAccount(t *testing.T) {
	manager := NewManager(t.TempDir(), "panel")
	migrated, err := manager.SaveIfMissing(Credentials{Username: "legacy", Password: "old", AuthorizationCompleted: true})
	if err != nil || !migrated {
		t.Fatalf("first migration: migrated=%v err=%v", migrated, err)
	}
	migrated, err = manager.SaveIfMissing(Credentials{Username: "other", Password: "new"})
	if err != nil || migrated {
		t.Fatalf("second migration: migrated=%v err=%v", migrated, err)
	}
	credentials, found, err := manager.Load()
	if err != nil || !found || credentials.Username != "legacy" || credentials.Password != "old" {
		t.Fatalf("credentials after migration = %#v found=%v err=%v", credentials, found, err)
	}
}

func TestManagerDownloadGateHonorsCancellation(t *testing.T) {
	manager := NewManager(t.TempDir(), "panel")
	release, err := manager.AcquireDownload(context.Background())
	if err != nil {
		t.Fatalf("acquire first gate: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := manager.AcquireDownload(ctx); err == nil {
		t.Fatal("second gate acquisition should time out")
	}
	release()
	secondRelease, err := manager.AcquireDownload(context.Background())
	if err != nil {
		t.Fatalf("acquire released gate: %v", err)
	}
	secondRelease()
}
