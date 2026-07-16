package web

import (
	"net/http"

	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
)

func (s *server) handleInstanceGameLanguage(w http.ResponseWriter, r *http.Request, instanceID string) {
	if r.Method == http.MethodGet {
		if _, ok := s.requireAdmin(w, r); !ok {
			return
		}
		instance, ok := s.loadInstance(w, r, instanceID)
		if !ok {
			return
		}
		settings, err := sj.ReadGameLanguageSettings(instance.DataDir)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "game_language_read_failed", sanitizeErrorMsg(err, "读取服务器游戏语言失败"))
			return
		}
		writeJSON(w, http.StatusOK, settings)
		return
	}

	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	var body sj.GameLanguageSettings
	if !decodeJSON(w, r, &body) {
		return
	}
	if err := sj.UpdateGameLanguageSettings(instance.DataDir, body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_game_language", sanitizeErrorMsg(err, "服务器游戏语言无效"))
		return
	}
	s.auditLog(r, &actor, "instance_game_language_update", "instance", instanceID, auditMetadata("languageCode", body.LanguageCode))
	writeJSON(w, http.StatusOK, body)
}
