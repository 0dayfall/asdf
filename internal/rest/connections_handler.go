package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/0dayfall/webfinger-server/internal/auth"
	"github.com/0dayfall/webfinger-server/internal/store"
)

type ConnectionsHandler struct {
	connStore store.ConnectionStore
	userStore auth.UserStore
}

func NewConnectionsHandler(connStore store.ConnectionStore, userStore auth.UserStore) *ConnectionsHandler {
	return &ConnectionsHandler{connStore: connStore, userStore: userStore}
}

// GET  /api/connections  — list accepted connections
// POST /api/connections  — send a request (body: {"username":"..."})
func (h *ConnectionsHandler) HandleConnections(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		conns, err := h.connStore.GetConnections(r.Context(), claims.UserID)
		if err != nil {
			http.Error(w, "failed to get connections", http.StatusInternalServerError)
			return
		}
		writeJSON(w, conns)

	case http.MethodPost:
		var req struct {
			Username string `json:"username"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Username) == "" {
			http.Error(w, "username required", http.StatusBadRequest)
			return
		}
		target, err := h.userStore.GetUserByUsername(r.Context(), req.Username)
		if err != nil || target == nil {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		if target.ID == claims.UserID {
			http.Error(w, "cannot connect to yourself", http.StatusBadRequest)
			return
		}
		if err := h.connStore.RequestConnection(r.Context(), claims.UserID, target.ID); err != nil {
			http.Error(w, "failed to send request", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, map[string]string{"status": "pending"})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// GET /api/connections/pending
func (h *ConnectionsHandler) HandlePending(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	claims := claimsFromContext(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	pending, err := h.connStore.GetPendingRequests(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "failed to get pending requests", http.StatusInternalServerError)
		return
	}
	writeJSON(w, pending)
}

// PUT    /api/connections/{id}/accept — accept a pending request (id = requesterID)
// DELETE /api/connections/{id}        — remove connection or cancel request
func (h *ConnectionsHandler) HandleConnectionByID(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := r.PathValue("id")
	otherID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut:
		// The other person sent the request; claims.UserID is the addressee.
		if err := h.connStore.AcceptConnection(r.Context(), otherID, claims.UserID); err != nil {
			http.Error(w, fmt.Sprintf("failed to accept: %v", err), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]string{"status": "accepted"})

	case http.MethodDelete:
		if err := h.connStore.RemoveConnection(r.Context(), claims.UserID, otherID); err != nil {
			http.Error(w, "failed to remove connection", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
