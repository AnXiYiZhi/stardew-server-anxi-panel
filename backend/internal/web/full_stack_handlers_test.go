package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

type scopedRuntimeProjection struct{}

func (scopedRuntimeProjection) EffectiveRuntimeInspection(instance registry.Instance) sjconfig.RuntimeStackInspection {
	if instance.ID == "confirmed" {
		return sjconfig.RuntimeStackInspection{Status: sjconfig.RuntimeStackStatusUpToDate, Code: "confirmed-content"}
	}
	return sjconfig.RuntimeStackInspection{Status: sjconfig.RuntimeStackStatusUpdateAvailable, Code: "outside-confirmation"}
}

func TestFullStackPublicRuntimeProjectionUsesInstanceBoundInspection(t *testing.T) {
	confirmed := runtimeInspectionFromDriver(scopedRuntimeProjection{}, registry.Instance{ID: "confirmed"})
	if confirmed.Status != sjconfig.RuntimeStackStatusUpToDate || confirmed.Available {
		t.Fatal("public update status discarded the proven instance exception")
	}
	other := runtimeInspectionFromDriver(scopedRuntimeProjection{}, registry.Instance{ID: "other"})
	if other.Status == sjconfig.RuntimeStackStatusUpToDate {
		t.Fatal("public projection inherited another world's exception")
	}
}

func TestFullStackRequestRejectsAmbiguousOrExecutableInputs(t *testing.T) {
	for _, body := range []string{
		`null`, `[]`, `{"action":"confirm","action":"prepare"}`,
		`{"action":"confirm","confirm":true,"Confirm":true}`,
		`{"action":"confirm","confirm":true,"confirm":false}`,
		`{"action":"confirm","members":["/data/other"]}`,
		`{"action":"confirm","image":"arbitrary:latest"}`,
		`{"action":"prepare","command":"shell"}`,
		`{"action":"prepare"} {}`, `{"action":42}`,
		`{"action":"prepare"}` + strings.Repeat(" ", 4096),
	} {
		request := httptest.NewRequest(http.MethodPost, "/api/system/update/full-stack", strings.NewReader(body))
		if _, err := decodeFullStackRequest(httptest.NewRecorder(), request); err == nil {
			t.Fatalf("ambiguous request accepted: %.100s", body)
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/api/system/update/full-stack", strings.NewReader(`{"action":"confirm","confirm":true,"operationId":"stack_exact","planSha256":"original"}`))
	body, err := decodeFullStackRequest(httptest.NewRecorder(), request)
	if err != nil || body.Action != "confirm" || !body.Confirm || body.OperationID != "stack_exact" || body.PlanSHA256 != "original" {
		t.Fatal("exact intent changed while decoding", err)
	}
}
