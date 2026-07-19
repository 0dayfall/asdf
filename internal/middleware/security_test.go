package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func corsRequest(t *testing.T, allowedOrigins []string, allowCredentials bool, origin string) *httptest.ResponseRecorder {
	t.Helper()
	handler := CORS(allowedOrigins, allowCredentials)(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) },
	))

	rr := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/api/profile", nil)
	require.NoError(t, err)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	handler.ServeHTTP(rr, req)
	return rr
}

func TestCORSWildcardNeverGrantsCredentials(t *testing.T) {
	// The vulnerable combination: wildcard allowlist + credentials enabled.
	// The middleware must NOT echo the caller's origin, and must NOT set
	// Allow-Credentials, otherwise any website can make authenticated
	// requests on behalf of a logged-in user.
	rr := corsRequest(t, []string{"*"}, true, "https://evil.example")

	require.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"),
		"wildcard must be sent literally, not echo the caller's origin")
	require.Empty(t, rr.Header().Get("Access-Control-Allow-Credentials"),
		"credentials must never be combined with a wildcard origin")
}

func TestCORSExplicitOriginGetsCredentials(t *testing.T) {
	rr := corsRequest(t, []string{"https://app.example.com"}, true, "https://app.example.com")

	require.Equal(t, "https://app.example.com", rr.Header().Get("Access-Control-Allow-Origin"))
	require.Equal(t, "true", rr.Header().Get("Access-Control-Allow-Credentials"))
}

func TestCORSUnlistedOriginGetsNothingWhenNoWildcard(t *testing.T) {
	rr := corsRequest(t, []string{"https://app.example.com"}, true, "https://evil.example")

	require.Empty(t, rr.Header().Get("Access-Control-Allow-Origin"))
	require.Empty(t, rr.Header().Get("Access-Control-Allow-Credentials"))
}

func TestCORSVariesOnOrigin(t *testing.T) {
	rr := corsRequest(t, []string{"*"}, false, "https://any.example")
	require.Contains(t, rr.Header().Values("Vary"), "Origin")
}
