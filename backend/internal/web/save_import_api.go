package web

import (
	"net/http"
	"strconv"
	"strings"

	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
)

const (
	hostModeSwapToPlayer        = "swap_to_player"
	hostModeVirtualHostTakeover = "virtual_host_takeover"
)

type saveImportHostHandlingRequest struct {
	Mode         string `json:"mode"`
	PlatformID   string `json:"platformId,omitempty"`
	Acknowledged bool   `json:"acknowledged,omitempty"`
}

type saveUploadCommitRequest struct {
	Token        string                         `json:"token"`
	Cancel       bool                           `json:"cancel,omitempty"`
	HostHandling *saveImportHostHandlingRequest `json:"hostHandling,omitempty"`
}

type saveUploadCommitResponse struct {
	JobID       string `json:"jobId"`
	OperationID string `json:"operationId"`
	SaveName    string `json:"saveName"`
}

func validateSaveImportHostHandling(input *saveImportHostHandlingRequest) (mode, driverMode, platformID, code string) {
	if input == nil {
		return "", "", "", "host_decision_required"
	}
	switch input.Mode {
	case hostModeSwapToPlayer:
		platformID = strings.TrimSpace(input.PlatformID)
		if platformID == "" {
			return "", "", "", "platform_id_invalid"
		}
		for _, ch := range platformID {
			if ch < '0' || ch > '9' {
				return "", "", "", "platform_id_invalid"
			}
		}
		value, err := strconv.ParseUint(platformID, 10, 64)
		if err != nil || value == 0 {
			return "", "", "", "platform_id_invalid"
		}
		return hostModeSwapToPlayer, "swap_host_to", platformID, ""
	case hostModeVirtualHostTakeover:
		if !input.Acknowledged {
			return "", "", "", "host_decision_required"
		}
		return hostModeVirtualHostTakeover, "server_owns_original", "", ""
	default:
		return "", "", "", "host_decision_required"
	}
}

func writeSaveImportSubmitError(w http.ResponseWriter, err error) {
	if typed, ok := sj.AsImportTransactionError(err); ok {
		switch typed.Code {
		case sj.ImportErrorUnsupported, sj.ImportErrorSaveExists, sj.ImportErrorBusy,
			sj.ImportErrorCommandFailed, sj.ImportErrorResultUnconfirmed,
			sj.ImportErrorRecoveryRequired, sj.ImportErrorActivationTimeout,
			sj.ImportErrorSaveInProgress:
			writeError(w, http.StatusConflict, typed.Code, typed.Message)
			return
		default:
			writeError(w, http.StatusConflict, sj.ImportErrorSaveInProgress, "save import transaction could not be started safely")
			return
		}
	}
	writeError(w, http.StatusInternalServerError, "import_transaction_failed", "failed to create import transaction")
}
