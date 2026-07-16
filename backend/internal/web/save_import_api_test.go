package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
)

func TestSaveImportHostHandlingValidation(t *testing.T) {
	tests := []struct {
		name       string
		input      *saveImportHostHandlingRequest
		wantMode   string
		wantDriver string
		wantID     string
		wantCode   string
	}{
		{name: "missing", wantCode: "host_decision_required"},
		{name: "takeover unacknowledged", input: &saveImportHostHandlingRequest{Mode: hostModeVirtualHostTakeover}, wantCode: "host_decision_required"},
		{name: "takeover", input: &saveImportHostHandlingRequest{Mode: hostModeVirtualHostTakeover, Acknowledged: true}, wantMode: hostModeVirtualHostTakeover, wantDriver: "server_owns_original"},
		{name: "swap empty", input: &saveImportHostHandlingRequest{Mode: hostModeSwapToPlayer}, wantCode: "platform_id_invalid"},
		{name: "swap sign", input: &saveImportHostHandlingRequest{Mode: hostModeSwapToPlayer, PlatformID: "+7656119"}, wantCode: "platform_id_invalid"},
		{name: "swap exponent", input: &saveImportHostHandlingRequest{Mode: hostModeSwapToPlayer, PlatformID: "7e16"}, wantCode: "platform_id_invalid"},
		{name: "swap zero", input: &saveImportHostHandlingRequest{Mode: hostModeSwapToPlayer, PlatformID: "000"}, wantCode: "platform_id_invalid"},
		{name: "swap overflow", input: &saveImportHostHandlingRequest{Mode: hostModeSwapToPlayer, PlatformID: "18446744073709551616"}, wantCode: "platform_id_invalid"},
		{name: "swap large string", input: &saveImportHostHandlingRequest{Mode: hostModeSwapToPlayer, PlatformID: " 18446744073709551615 "}, wantMode: hostModeSwapToPlayer, wantDriver: "swap_host_to", wantID: "18446744073709551615"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mode, driverMode, platformID, code := validateSaveImportHostHandling(tc.input)
			if mode != tc.wantMode || driverMode != tc.wantDriver || platformID != tc.wantID || code != tc.wantCode {
				t.Fatalf("got mode=%q driver=%q id=%q code=%q", mode, driverMode, platformID, code)
			}
		})
	}
}

func TestSaveImportStableErrorMapping(t *testing.T) {
	codes := []string{
		sj.ImportErrorSaveExists,
		sj.ImportErrorUnsupported,
		sj.ImportErrorBusy,
		sj.ImportErrorCommandFailed,
		sj.ImportErrorResultUnconfirmed,
		sj.ImportErrorRecoveryRequired,
		sj.ImportErrorActivationTimeout,
		sj.ImportErrorSaveInProgress,
	}
	for _, code := range codes {
		t.Run(code, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			writeSaveImportSubmitError(recorder, &sj.ImportTransactionError{Code: code, Message: "safe"})
			if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), `"code":"`+code+`"`) {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
