package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
)

func TestWriteStardewMutationGuardConflict(t *testing.T) {
	recorder := httptest.NewRecorder()
	if !writeStardewMutationGuardConflict(recorder, &sj.NewGameOwnerError{Code: "new_game_in_progress", Message: "owner active"}) {
		t.Fatal("typed owner error was not handled")
	}
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), `"code":"new_game_in_progress"`) {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
	if writeStardewMutationGuardConflict(httptest.NewRecorder(), errors.New("unrelated")) {
		t.Fatal("unrelated error was consumed")
	}
}
