package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/0dayfall/webfinger-server/internal/auth"
	"github.com/0dayfall/webfinger-server/internal/store"
)

type LinksHandler struct {
	linkStore store.LinkStore
}

func NewLinksHandler(linkStore store.LinkStore) *LinksHandler {
	return &LinksHandler{linkStore: linkStore}
}

// GET  /api/profile/links — list caller's links
// POST /api/profile/links — add a link
func (h *LinksHandler) HandleLinks(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		links, err := h.linkStore.GetUserLinks(r.Context(), claims.UserID)
		if err != nil {
			http.Error(w, "failed to get links", http.StatusInternalServerError)
			return
		}
		writeJSON(w, links)

	case http.MethodPost:
		var link store.UserLink
		if err := json.NewDecoder(r.Body).Decode(&link); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		link.UserID = claims.UserID
		if err := validateLink(&link); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		created, err := h.linkStore.AddLink(r.Context(), &link)
		if err != nil {
			http.Error(w, "failed to add link", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, created)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// PUT    /api/profile/links/{id} — update
// DELETE /api/profile/links/{id} — delete
func (h *LinksHandler) HandleLinkByID(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut:
		var link store.UserLink
		if err := json.NewDecoder(r.Body).Decode(&link); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if err := validateLink(&link); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		updated, err := h.linkStore.UpdateLink(r.Context(), id, claims.UserID, &link)
		if err != nil {
			http.Error(w, "failed to update link", http.StatusInternalServerError)
			return
		}
		writeJSON(w, updated)

	case http.MethodDelete:
		if err := h.linkStore.DeleteLink(r.Context(), id, claims.UserID); err != nil {
			http.Error(w, "failed to delete link", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

var validVisibilities = map[string]bool{
	"public": true, "registered": true, "friends": true, "private": true,
}

func validateLink(l *store.UserLink) error {
	if l.Href == "" {
		return fmt.Errorf("href is required")
	}
	if l.Visibility == "" {
		l.Visibility = "public"
	}
	if !validVisibilities[l.Visibility] {
		return fmt.Errorf("visibility must be public, registered, friends, or private")
	}
	return nil
}

func claimsFromContext(r *http.Request) *auth.Claims {
	v, _ := r.Context().Value("user").(*auth.Claims)
	return v
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
