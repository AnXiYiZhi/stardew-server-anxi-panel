package web

import (
	"encoding/json"
	"net/http"
)

// NewHandler returns the HTTP routes for the panel backend.
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	return mux
}

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(healthResponse{
		Status:  "ok",
		Service: "stardew-anxi-panel",
	}); err != nil {
		http.Error(w, "failed to write health response", http.StatusInternalServerError)
	}
}
