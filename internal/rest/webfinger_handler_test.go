package rest

import (
	"asdf/internal/api"
	"asdf/internal/db"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGETResource(t *testing.T) {
	t.Run("Test the web handler", func(t *testing.T) {
		// Arrange
		db := db.NewData()
		err := db.LoadData(path.Join("test", "data.json"))
		require.NoError(t, err)
		wfh := WebFingerHandler{Data: db}

		rr := httptest.NewRecorder()
		request, err := http.NewRequest(http.MethodGet, "/.well-known/webfinger?resource=acct:example@example.com", nil)
		require.NoError(t, err)

		// Act
		wfh.ServeHTTP(rr, request)

		// Assert
		require.EqualValues(t, http.StatusOK, rr.Code)
		require.EqualValues(t, "application/jrd+json", rr.Header().Get("Content-Type"))

		body, err := io.ReadAll(rr.Body)
		require.NoError(t, err)

		var jrd api.JRD
		err = json.Unmarshal(body, &jrd)
		require.NoError(t, err)

		var expected api.JRD
		expectedJSON := `{"subject":"acct:example@example.com","aliases":["http://example.com/profile/example"],"properties":{"http://example.com/prop/name":"Example User"},"links":[{"rel":"http://webfinger.net/rel/profile-page","type":"text/html","href":"http://example.com/profile/example"},{"rel":"http://example.com/rel/blog","type":"text/html","href":"http://blogs.example.com/example/"}]}`
		err = json.Unmarshal([]byte(expectedJSON), &expected)
		require.NoError(t, err)

		require.EqualValues(t, expected, jrd)
	})
}
