package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/facebook"
)

// OAuthStateData is stored in Redis keyed by the random state token.
type OAuthStateData struct {
	Provider     string `json:"provider"`
	CodeVerifier string `json:"code_verifier,omitempty"` // PKCE for X
	ReturnTo     string `json:"return_to,omitempty"`
}

// OAuthUserInfo is the normalised profile from any provider.
type OAuthUserInfo struct {
	ProviderUserID   string
	ProviderUsername string
	DisplayName      string
	Email            string
}

type OAuthService struct {
	facebook  *oauth2.Config
	instagram *oauth2.Config
	x         *oauth2.Config // X uses PKCE — handled manually
	baseURL   string
}

func NewOAuthService(baseURL, fbClientID, fbSecret, igClientID, igSecret, xClientID, xSecret string) *OAuthService {
	s := &OAuthService{baseURL: strings.TrimRight(baseURL, "/")}

	if fbClientID != "" {
		s.facebook = &oauth2.Config{
			ClientID:     fbClientID,
			ClientSecret: fbSecret,
			RedirectURL:  s.baseURL + "/auth/facebook/callback",
			Scopes:       []string{"public_profile", "email"},
			Endpoint:     facebook.Endpoint,
		}
	}

	if igClientID != "" {
		s.instagram = &oauth2.Config{
			ClientID:     igClientID,
			ClientSecret: igSecret,
			RedirectURL:  s.baseURL + "/auth/instagram/callback",
			Scopes:       []string{"user_profile", "user_media"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://api.instagram.com/oauth/authorize",
				TokenURL: "https://api.instagram.com/oauth/access_token",
			},
		}
	}

	if xClientID != "" {
		s.x = &oauth2.Config{
			ClientID:     xClientID,
			ClientSecret: xSecret,
			RedirectURL:  s.baseURL + "/auth/x/callback",
			Scopes:       []string{"tweet.read", "users.read"},
			Endpoint: oauth2.Endpoint{
				AuthURL:   "https://twitter.com/i/oauth2/authorize",
				TokenURL:  "https://api.twitter.com/2/oauth2/token",
				AuthStyle: oauth2.AuthStyleInHeader,
			},
		}
	}

	return s
}

func (s *OAuthService) IsEnabled(provider string) bool {
	switch provider {
	case "facebook":
		return s.facebook != nil
	case "instagram":
		return s.instagram != nil
	case "x":
		return s.x != nil
	case "bluesky":
		return true // no credentials needed (uses app passwords)
	}
	return false
}

// AuthURL builds the redirect URL for a provider.
// For X it also returns a PKCE code verifier to store alongside the state.
func (s *OAuthService) AuthURL(provider, state string) (authURL, codeVerifier string, err error) {
	switch provider {
	case "facebook":
		if s.facebook == nil {
			return "", "", fmt.Errorf("facebook not configured")
		}
		return s.facebook.AuthCodeURL(state, oauth2.AccessTypeOnline), "", nil

	case "instagram":
		if s.instagram == nil {
			return "", "", fmt.Errorf("instagram not configured")
		}
		return s.instagram.AuthCodeURL(state), "", nil

	case "x":
		if s.x == nil {
			return "", "", fmt.Errorf("x not configured")
		}
		verifier := oauth2.GenerateVerifier()
		u := s.x.AuthCodeURL(state,
			oauth2.S256ChallengeOption(verifier),
			oauth2.AccessTypeOnline,
		)
		return u, verifier, nil
	}
	return "", "", fmt.Errorf("unknown provider: %s", provider)
}

// ExchangeCode exchanges an auth code for a token and returns normalised user info.
func (s *OAuthService) ExchangeCode(ctx context.Context, provider, code, codeVerifier string) (*OAuthUserInfo, error) {
	switch provider {
	case "facebook":
		return s.exchangeFacebook(ctx, code)
	case "instagram":
		return s.exchangeInstagram(ctx, code)
	case "x":
		return s.exchangeX(ctx, code, codeVerifier)
	}
	return nil, fmt.Errorf("unknown provider: %s", provider)
}

func (s *OAuthService) exchangeFacebook(ctx context.Context, code string) (*OAuthUserInfo, error) {
	tok, err := s.facebook.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("facebook token exchange: %w", err)
	}
	client := s.facebook.Client(ctx, tok)
	resp, err := client.Get("https://graph.facebook.com/me?fields=id,name,email")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var data struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return &OAuthUserInfo{
		ProviderUserID:   data.ID,
		ProviderUsername: data.ID,
		DisplayName:      data.Name,
		Email:            data.Email,
	}, nil
}

func (s *OAuthService) exchangeInstagram(ctx context.Context, code string) (*OAuthUserInfo, error) {
	tok, err := s.instagram.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("instagram token exchange: %w", err)
	}
	client := s.instagram.Client(ctx, tok)
	resp, err := client.Get("https://graph.instagram.com/me?fields=id,username")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var data struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return &OAuthUserInfo{
		ProviderUserID:   data.ID,
		ProviderUsername: data.Username,
		DisplayName:      data.Username,
	}, nil
}

func (s *OAuthService) exchangeX(ctx context.Context, code, codeVerifier string) (*OAuthUserInfo, error) {
	tok, err := s.x.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		return nil, fmt.Errorf("x token exchange: %w", err)
	}
	client := s.x.Client(ctx, tok)
	resp, err := client.Get("https://api.twitter.com/2/users/me?user.fields=name,username")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var data struct {
		Data struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Username string `json:"username"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return &OAuthUserInfo{
		ProviderUserID:   data.Data.ID,
		ProviderUsername: data.Data.Username,
		DisplayName:      data.Data.Name,
	}, nil
}

// BlueskyLogin authenticates via the AT Protocol using a handle + app password.
func BlueskyLogin(ctx context.Context, handle, appPassword string) (*OAuthUserInfo, error) {
	body, _ := json.Marshal(map[string]string{
		"identifier": handle,
		"password":   appPassword,
	})
	resp, err := http.Post(
		"https://bsky.social/xrpc/com.atproto.server.createSession",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("bluesky request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bluesky auth failed: %s", string(b))
	}

	var data struct {
		DID    string `json:"did"`
		Handle string `json:"handle"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return &OAuthUserInfo{
		ProviderUserID:   data.DID,
		ProviderUsername: data.Handle,
		DisplayName:      data.Handle,
	}, nil
}

// GenerateState creates a cryptographically random state string.
func GenerateState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// FindOrCreateOAuthUser finds an existing user by OAuth account or creates a new one.
func FindOrCreateOAuthUser(
	ctx context.Context,
	oauthStore OAuthStore,
	userStore UserStore,
	authService *AuthService,
	provider string,
	info *OAuthUserInfo,
	domain string,
) (*User, error) {
	// 1. Existing OAuth account?
	acc, err := oauthStore.FindAccount(ctx, provider, info.ProviderUserID)
	if err != nil {
		return nil, err
	}
	if acc != nil {
		return userStore.GetUserByID(ctx, acc.UserID)
	}

	// 2. Existing user with matching email?
	var user *User
	if info.Email != "" {
		user, _ = userStore.GetUserByEmail(ctx, info.Email)
	}

	// 3. Create new user
	if user == nil {
		username := sanitiseUsername(info.ProviderUsername)
		// Ensure uniqueness
		base := username
		for i := 1; i < 100; i++ {
			existing, _ := userStore.GetUserByUsername(ctx, username)
			if existing == nil {
				break
			}
			username = fmt.Sprintf("%s%d", base, i)
		}
		user, err = userStore.CreateUser(ctx, &CreateUserRequest{
			Username:    username,
			Email:       info.Email,
			Password:    "", // no password for OAuth-only accounts
			DisplayName: info.DisplayName,
			Domain:      domain,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create oauth user: %w", err)
		}
	}

	// 4. Link the OAuth account
	if err := oauthStore.UpsertAccount(ctx, &OAuthAccount{
		UserID:              user.ID,
		Provider:            provider,
		ProviderUserID:      info.ProviderUserID,
		ProviderUsername:    info.ProviderUsername,
		ProviderDisplayName: info.DisplayName,
		TokenExpiresAt:      timePtr(time.Now().Add(60 * 24 * time.Hour)),
	}); err != nil {
		return nil, err
	}

	return user, nil
}

func sanitiseUsername(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		}
	}
	u := b.String()
	if len(u) < 3 {
		u = u + "user"
	}
	if len(u) > 30 {
		u = u[:30]
	}
	return u
}

func buildCallbackURL(baseURL, provider string) string {
	u, _ := url.JoinPath(baseURL, "/auth/"+provider+"/callback")
	return u
}

func timePtr(t time.Time) *time.Time { return &t }
