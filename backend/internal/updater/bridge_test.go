package updater

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBridgeReceiptUpgradeAndFreshInstall(t *testing.T) {
	root := t.TempDir()
	if BridgeReady(root, "0.8.0") == nil {
		t.Fatal("skipped bridge accepted")
	}
	if err := RecordBridgeReady(root, "0.7.3", false); err != nil {
		t.Fatal(err)
	}
	if err := BridgeReady(root, "0.8.0"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bridgePath(root), []byte(`{"schema":1,"origin":"bridge","version":"0.7.2"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if BridgeReady(root, "0.8.0") == nil {
		t.Fatal("invalid receipt accepted")
	}
	fresh := t.TempDir()
	if err := RecordBridgeReady(fresh, "0.8.0", true); err != nil {
		t.Fatal(err)
	}
	if err := BridgeReady(fresh, "0.8.0"); err != nil {
		t.Fatal(err)
	}
}

func TestPendingLegacyUpgradeCannotClaimFreshInstall(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "updater"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "updater", "apply-status.json"), []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if RecordBridgeReady(root, "0.8.0", true) == nil {
		t.Fatal("pending bypass accepted")
	}
	if validateUpgradeTransition("0.7.2", "0.8.0") == nil {
		t.Fatal("direct updater bypass accepted")
	}
}
