package handlers

import (
	"net/http"
	"strings"
	"webhook-dispatcher/internal/store"
)

type EventsHandler struct {
	Store *store.EventStore
}

func (h *EventsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Route: GET /events -> list all
	// Route: GET /events/{id} -> get one
	id := strings.TrimPrefix(r.URL.Path, "/events/")
	id = strings.TrimPrefix(id, "/events/")
	id = strings.Trim(id, "/")

	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Only GET supported")
		return
	}

	if id == "" {
		writeJSON(w, http.StatusOK, h.Store.All())
		return
	}

	record, ok := h.Store.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "Event not found")
		return
	}

	writeJSON(w, http.StatusOK, record)
}