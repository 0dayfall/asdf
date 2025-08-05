//go:build integration
// +build integration

package rest

import (
	"asdf/internal/store"
	"asdf/internal/types"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGETResource(t *testing.T) {
	t.Run("Returns a known user", func(t *testing.T) {
		// Arrange
		mockStore := store.NewMockStore()
		mockStore.Add(&types.JRD{
			Subject: "acct:example@example.com",
			Aliases: []string{"http://example.com/profile/example"},
			Properties: map[string]interface{}{
				"http://example.com/prop/name": "Example User",
			},
			Links: []types.Link{
				{Rel: "http://webfinger.net/rel/profile-page", Type: "text/html", Href: "http://example.com/profile/example"},
				{Rel: "http://example.com/rel/blog", Type: "text/html", Href: "http://blogs.example.com/example/"},
			},
		})

		wfh := WebFingerHandler{Data: mockStore}

		rr := httptest.NewRecorder()
		request, err := http.NewRequest(http.MethodGet, "/.well-known/webfinger?resource=acct:example@example.com", nil)
		require.NoError(t, err)

		// Act
		wfh.ServeHTTP(rr, request)

		// Assert
		require.Equal(t, http.StatusOK, rr.Code)
		require.Equal(t, "application/jrd+json", rr.Header().Get("Content-Type"))

		var jrd types.JRD
		err = json.NewDecoder(rr.Body).Decode(&jrd)
		require.NoError(t, err)

		require.Equal(t, "acct:example@example.com", jrd.Subject)
		require.Equal(t, "Example User", jrd.Properties["http://example.com/prop/name"])
	})
}
