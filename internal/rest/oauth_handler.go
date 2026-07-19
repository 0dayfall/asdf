package rest

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/0dayfall/webfinger-server/internal/auth"
	"github.com/0dayfall/webfinger-server/internal/cache"
)

type OAuthHandler struct {
	oauthService *auth.OAuthService
	oauthStore   auth.OAuthStore
	userStore    auth.UserStore
	authService  *auth.AuthService
	cache        cache.Cache
	domain       string
}

func NewOAuthHandler(
	oauthService *auth.OAuthService,
	oauthStore auth.OAuthStore,
	userStore auth.UserStore,
	authService *auth.AuthService,
	c cache.Cache,
	domain string,
) *OAuthHandler {
	return &OAuthHandler{
		oauthService: oauthService,
		oauthStore:   oauthStore,
		userStore:    userStore,
		authService:  authService,
		cache:        c,
		domain:       domain,
	}
}

// HandleOAuthRedirect initiates an OAuth flow by redirecting to the provider.
// GET /auth/{provider}
func (h *OAuthHandler) HandleOAuthRedirect(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	if !h.oauthService.IsEnabled(provider) {
		http.Error(w, "provider not available", http.StatusBadRequest)
		return
	}

	state, err := auth.GenerateState()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	authURL, codeVerifier, err := h.oauthService.AuthURL(provider, state)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	returnTo := r.URL.Query().Get("return_to")
	stateData := auth.OAuthStateData{
		Provider:     provider,
		CodeVerifier: codeVerifier,
		ReturnTo:     returnTo,
	}
	stateBytes, _ := json.Marshal(stateData)

	if h.cache == nil {
		http.Error(w, "OAuth requires Redis cache", http.StatusServiceUnavailable)
		return
	}
	if err := h.cache.SetOAuthState(r.Context(), state, stateBytes, 15*time.Minute); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, authURL, http.StatusFound)
}

// HandleOAuthCallback handles the provider callback after user authorises.
// GET /auth/{provider}/callback
func (h *OAuthHandler) HandleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		http.Error(w, "missing code or state", http.StatusBadRequest)
		return
	}

	if h.cache == nil {
		http.Error(w, "OAuth requires Redis cache", http.StatusServiceUnavailable)
		return
	}
	stateBytes, err := h.cache.GetOAuthState(r.Context(), state)
	if err != nil || stateBytes == nil {
		http.Error(w, "invalid or expired state", http.StatusBadRequest)
		return
	}
	_ = h.cache.DeleteOAuthState(r.Context(), state)

	var stateData auth.OAuthStateData
	if err := json.Unmarshal(stateBytes, &stateData); err != nil || stateData.Provider != provider {
		http.Error(w, "state mismatch", http.StatusBadRequest)
		return
	}

	info, err := h.oauthService.ExchangeCode(r.Context(), provider, code, stateData.CodeVerifier)
	if err != nil {
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}

	user, err := auth.FindOrCreateOAuthUser(r.Context(), h.oauthStore, h.userStore, h.authService, provider, info, h.domain)
	if err != nil {
		http.Error(w, "failed to create account", http.StatusInternalServerError)
		return
	}

	token, err := h.authService.GenerateToken(r.Context(), user, getClientIP(r), r.Header.Get("User-Agent"))
	if err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	// Pass token via URL fragment — never sent to the server, not accessible to other origins.
	returnTo := stateData.ReturnTo
	if returnTo == "" {
		returnTo = "/dashboard"
	}
	http.Redirect(w, r, returnTo+"#token="+token, http.StatusFound)
}

// HandleBlueskyLogin authenticates via AT Protocol handle + app password.
// POST /auth/bluesky
func (h *OAuthHandler) HandleBlueskyLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Handle      string `json:"handle"`
		AppPassword string `json:"app_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Handle == "" || req.AppPassword == "" {
		http.Error(w, "handle and app_password required", http.StatusBadRequest)
		return
	}

	info, err := auth.BlueskyLogin(r.Context(), req.Handle, req.AppPassword)
	if err != nil {
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}

	user, err := auth.FindOrCreateOAuthUser(r.Context(), h.oauthStore, h.userStore, h.authService, "bluesky", info, h.domain)
	if err != nil {
		http.Error(w, "failed to create account", http.StatusInternalServerError)
		return
	}

	token, err := h.authService.GenerateToken(r.Context(), user, getClientIP(r), r.Header.Get("User-Agent"))
	if err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"token":      token,
		"expires_at": time.Now().Add(24 * time.Hour),
	})
}
