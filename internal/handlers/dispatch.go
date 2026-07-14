package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"webhook-dispatcher/internal/config"
	"webhook-dispatcher/internal/dispatcher"
	"webhook-dispatcher/internal/models"
	"webhook-dispatcher/internal/store"
)

type DispatchHandler struct {
	Store 	*store.EventStore
	Client 	*http.Client
	Cfg 	config.Config
}

func(h *DispatchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Only POST supported")
		return
	}

	var req models.DispatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}
	if len(req.Destinations) == 0 {
		writeError(w, http.StatusBadRequest, "At least one destination required")
		return
	}
	if req.Payload == nil {
		writeError(w, http.StatusBadRequest, "Payload required")
		return
	}

	eventID, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to generate event ID: "+err.Error())
		return
	}

	resp := dispatcher.Dispatch(
		r.Context(), req, eventID, h.Client, h.Store,
		h.Cfg.MaxAttempts, h.Cfg.BackoffBase, h.Cfg.BackoffMax, h.Cfg.DeliveryTimeout,
	)
	writeJSON(w, http.StatusOK, resp)
}

func randomID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
