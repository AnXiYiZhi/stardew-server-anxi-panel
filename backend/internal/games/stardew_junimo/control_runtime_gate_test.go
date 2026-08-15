package stardew_junimo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

func setupControlRuntimeGateTest(t *testing.T) (string, string) {
	t.Helper()
	dataDir := t.TempDir()
	if err := installSMAPIMod(dataDir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(controlDir(dataDir), 0o700); err != nil {
		t.Fatal(err)
	}
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatal(err)
	}
	return dataDir, manifest.Control.Version
}

func writeControlRuntimeOptions(t *testing.T, dataDir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(controlDir(dataDir), "options.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestInspectControlRuntimeGateClassifiesRuntimeEvidence(t *testing.T) {
	tests := []struct {
		name       string
		prepare    func(*testing.T, string, string)
		wantState  ControlRuntimeGateState
		wantCode   string
		wantActual string
	}{
		{
			name:      "missing options is pending",
			wantState: ControlRuntimeGatePending,
			wantCode:  ControlRuntimeCodePending,
		},
		{
			name: "current runtime is ready",
			prepare: func(t *testing.T, dataDir, expected string) {
				writeControlRuntimeOptions(t, dataDir, `{"controlModVersion":"`+expected+`","hostFarmhousePreservationPatchAvailable":true}`)
			},
			wantState: ControlRuntimeGateReady,
			wantCode:  ControlRuntimeCodeReady,
		},
		{
			name: "missing host farmhouse patch evidence is invalid",
			prepare: func(t *testing.T, dataDir, expected string) {
				writeControlRuntimeOptions(t, dataDir, `{"controlModVersion":"`+expected+`"}`)
			},
			wantState: ControlRuntimeGateInvalid,
			wantCode:  ControlRuntimeCodeHostFarmhousePatchUnavailable,
		},
		{
			name: "failed host farmhouse patch is invalid",
			prepare: func(t *testing.T, dataDir, expected string) {
				writeControlRuntimeOptions(t, dataDir, `{"controlModVersion":"`+expected+`","hostFarmhousePreservationPatchAvailable":false}`)
			},
			wantState: ControlRuntimeGateInvalid,
			wantCode:  ControlRuntimeCodeHostFarmhousePatchUnavailable,
		},
		{
			name: "explicit old runtime is mismatch",
			prepare: func(t *testing.T, dataDir, _ string) {
				writeControlRuntimeOptions(t, dataDir, `{"controlModVersion":"0.2.2"}`)
			},
			wantState:  ControlRuntimeGateVersionMismatch,
			wantCode:   ControlRuntimeCodeVersionMismatch,
			wantActual: "0.2.2",
		},
		{
			name: "malformed options is invalid",
			prepare: func(t *testing.T, dataDir, _ string) {
				writeControlRuntimeOptions(t, dataDir, `{"controlModVersion":`)
			},
			wantState: ControlRuntimeGateInvalid,
			wantCode:  ControlRuntimeCodeOptionsInvalid,
		},
		{
			name: "empty runtime version is invalid",
			prepare: func(t *testing.T, dataDir, _ string) {
				writeControlRuntimeOptions(t, dataDir, `{"controlModVersion":""}`)
			},
			wantState: ControlRuntimeGateInvalid,
			wantCode:  ControlRuntimeCodeOptionsInvalid,
		},
		{
			name: "missing installed dll is invalid",
			prepare: func(t *testing.T, dataDir, _ string) {
				if err := os.Remove(filepath.Join(smapiModDir(dataDir), "StardewAnxiPanel.Control.dll")); err != nil {
					t.Fatal(err)
				}
			},
			wantState: ControlRuntimeGateInvalid,
			wantCode:  ControlRuntimeCodeDLLMissing,
		},
		{
			name: "changed installed dll is invalid",
			prepare: func(t *testing.T, dataDir, _ string) {
				if err := os.WriteFile(filepath.Join(smapiModDir(dataDir), "StardewAnxiPanel.Control.dll"), []byte("changed"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			wantState: ControlRuntimeGateInvalid,
			wantCode:  ControlRuntimeCodeDLLHashMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir, expected := setupControlRuntimeGateTest(t)
			if tt.prepare != nil {
				tt.prepare(t, dataDir, expected)
			}
			got := InspectControlRuntimeGate(dataDir)
			wantActual := tt.wantActual
			if tt.wantState == ControlRuntimeGateReady || tt.wantCode == ControlRuntimeCodeHostFarmhousePatchUnavailable {
				wantActual = expected
			}
			if got.State != tt.wantState || got.Code != tt.wantCode || got.Expected != expected || got.Actual != wantActual {
				t.Fatalf("InspectControlRuntimeGate() = %+v, want state=%q code=%q expected=%q actual=%q", got, tt.wantState, tt.wantCode, expected, wantActual)
			}
		})
	}
}

func TestWaitForControlRuntimeGateAcceptsDelayedOptions(t *testing.T) {
	dataDir, expected := setupControlRuntimeGateTest(t)
	writeDone := make(chan struct{})
	go func() {
		time.Sleep(25 * time.Millisecond)
		_ = os.WriteFile(filepath.Join(controlDir(dataDir), "options.json"), []byte(`{"controlModVersion":"`+expected+`","hostFarmhousePreservationPatchAvailable":true}`), 0o600)
		close(writeDone)
	}()

	got, err := waitForControlRuntimeGate(context.Background(), dataDir, time.Second)
	<-writeDone
	if err != nil || got.State != ControlRuntimeGateReady {
		t.Fatalf("waitForControlRuntimeGate() = %+v, %v; want ready", got, err)
	}
}

func TestWaitForControlRuntimeGateReturnsExplicitMismatchImmediately(t *testing.T) {
	dataDir, _ := setupControlRuntimeGateTest(t)
	writeControlRuntimeOptions(t, dataDir, `{"controlModVersion":"0.2.2"}`)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	got, err := waitForControlRuntimeGate(ctx, dataDir, time.Second)
	if err != nil || got.State != ControlRuntimeGateVersionMismatch || got.Actual != "0.2.2" {
		t.Fatalf("waitForControlRuntimeGate() = %+v, %v; want explicit mismatch", got, err)
	}
}

func TestWaitForControlRuntimeGatePreservesContextCancellation(t *testing.T) {
	dataDir, _ := setupControlRuntimeGateTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := waitForControlRuntimeGate(ctx, dataDir, time.Second)
	if !errors.Is(err, context.Canceled) || got.State != ControlRuntimeGatePending {
		t.Fatalf("waitForControlRuntimeGate() = %+v, %v; want pending/context canceled", got, err)
	}
}
