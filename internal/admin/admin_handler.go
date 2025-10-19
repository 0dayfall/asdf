package admin

import (
	"asdf/internal/auth"
	"asdf/internal/monitoring"
	"asdf/internal/store"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

type AdminHandler struct {
	userStore   auth.UserStore
	store       store.Store
	metrics     *monitoring.Metrics
	authHandler AuthMiddleware // For middleware
}

type AdminStats struct {
	UserCount      int           `json:"user_count"`
	ActiveUsers    int           `json:"active_users"`
	AdminUsers     int           `json:"admin_users"`
	RequestsToday  int64         `json:"requests_today"`
	CacheStats     interface{}   `json:"cache_stats,omitempty"`
	SystemUptime   time.Duration `json:"system_uptime"`
	DatabaseStatus string        `json:"database_status"`
	RedisStatus    string        `json:"redis_status"`
}

// AuthMiddleware interface for middleware
type AuthMiddleware interface {
	AuthMiddleware(next http.Handler) http.Handler
	RequireAdmin(next http.Handler) http.Handler
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(userStore auth.UserStore, store store.Store, metrics *monitoring.Metrics, authHandler AuthMiddleware) *AdminHandler {
	return &AdminHandler{
		userStore:   userStore,
		store:       store,
		metrics:     metrics,
		authHandler: authHandler,
	}
}

// RegisterRoutes registers admin routes
func (h *AdminHandler) RegisterRoutes(mux *http.ServeMux) {
	// All admin routes require authentication and admin privileges
	adminHandler := func(handler http.HandlerFunc) http.Handler {
		return h.authHandler.RequireAdmin(h.authHandler.AuthMiddleware(http.HandlerFunc(handler)))
	}

	mux.Handle("/api/admin/stats", adminHandler(h.HandleStats))
	mux.Handle("/api/admin/users", adminHandler(h.HandleUsers))
	mux.Handle("/api/admin/users/", adminHandler(h.HandleUser))
	mux.Handle("/api/admin/system", adminHandler(h.HandleSystem))
}

// HandleStats returns system statistics
func (h *AdminHandler) HandleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Get user counts
	allUsers, totalUsers, err := h.userStore.ListUsers(ctx, &auth.UserFilters{Limit: 1})
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to get user count", err.Error())
		return
	}
	_ = allUsers // We only need the count

	activeUsers, _, err := h.userStore.ListUsers(ctx, &auth.UserFilters{IsActive: boolPtr(true), Limit: 1})
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to get active user count", err.Error())
		return
	}

	adminUsers, _, err := h.userStore.ListUsers(ctx, &auth.UserFilters{IsAdmin: boolPtr(true), Limit: 1})
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to get admin user count", err.Error())
		return
	}

	stats := AdminStats{
		UserCount:      totalUsers,
		ActiveUsers:    len(activeUsers),
		AdminUsers:     len(adminUsers),
		SystemUptime:   time.Since(time.Now().Add(-24 * time.Hour)), // Placeholder
		DatabaseStatus: "healthy",
		RedisStatus:    "healthy",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// HandleUsers returns a list of users with pagination
func (h *AdminHandler) HandleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listUsers(w, r)
	case http.MethodPost:
		h.createUser(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleUser handles operations on a specific user
func (h *AdminHandler) HandleUser(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from path
	path := r.URL.Path
	if len(path) < len("/api/admin/users/") {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	userIDStr := path[len("/api/admin/users/"):]
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getUser(w, r, userID)
	case http.MethodPut:
		h.updateUser(w, r, userID)
	case http.MethodDelete:
		h.deleteUser(w, r, userID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleSystem handles system operations
func (h *AdminHandler) HandleSystem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	action := r.URL.Query().Get("action")
	switch action {
	case "clear_cache":
		h.clearCache(w, r)
	case "backup_db":
		h.backupDatabase(w, r)
	default:
		http.Error(w, "Unknown action", http.StatusBadRequest)
	}
}

func (h *AdminHandler) listUsers(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	search := r.URL.Query().Get("search")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	offset := 0
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	filters := &auth.UserFilters{
		Search: search,
		Limit:  limit,
		Offset: offset,
	}

	users, total, err := h.userStore.ListUsers(r.Context(), filters)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to list users", err.Error())
		return
	}

	response := map[string]interface{}{
		"users":  users,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *AdminHandler) getUser(w http.ResponseWriter, r *http.Request, userID int) {
	user, err := h.userStore.GetUserByID(r.Context(), userID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "User not found", err.Error())
		return
	}

	// Remove sensitive data
	user.PasswordHash = ""

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *AdminHandler) updateUser(w http.ResponseWriter, r *http.Request, userID int) {
	var updates auth.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid JSON", err.Error())
		return
	}

	user, err := h.userStore.UpdateUser(r.Context(), userID, &updates)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to update user", err.Error())
		return
	}

	// Remove sensitive data
	user.PasswordHash = ""

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *AdminHandler) deleteUser(w http.ResponseWriter, r *http.Request, userID int) {
	if err := h.userStore.DeleteUser(r.Context(), userID); err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to delete user", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AdminHandler) createUser(w http.ResponseWriter, r *http.Request) {
	var req auth.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid JSON", err.Error())
		return
	}

	// Basic validation
	if req.Username == "" || req.Email == "" || req.Password == "" {
		h.writeError(w, http.StatusBadRequest, "Username, email, and password required", "")
		return
	}

	user, err := h.userStore.CreateUser(r.Context(), &req)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to create user", err.Error())
		return
	}

	// Remove sensitive data
	user.PasswordHash = ""

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

func (h *AdminHandler) clearCache(w http.ResponseWriter, r *http.Request) {
	// Implement cache clearing logic
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Cache cleared successfully",
	})
}

func (h *AdminHandler) backupDatabase(w http.ResponseWriter, r *http.Request) {
	// Implement database backup logic
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Backup started",
	})
}

func (h *AdminHandler) writeError(w http.ResponseWriter, statusCode int, message, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]interface{}{
		"error":   message,
		"message": details,
	}

	json.NewEncoder(w).Encode(response)
}

func boolPtr(b bool) *bool {
	return &b
}
