package rest

import (
	"asdf/internal/api"
	"asdf/internal/db"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFinger(t *testing.T) {
	// Arrange
	db := db.NewData()

	err := db.LoadData(path.Join("test", "data.json"))
	require.NoError(t, err)

	req, err := http.NewRequest("GET", "/.well-known/webfinger?resource=acct%3Aexample%40example.com", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	wfh := WebFingerHandler{Data: db}

	// Act
	wfh.ServeHTTP(rr, req)

	// Assert
	require.EqualValues(t, http.StatusOK, rr.Code)
	require.EqualValues(t, "application/jrd+json", rr.Header().Get("Content-Type"))

	var jrd api.JRD
	err = json.Unmarshal(rr.Body.Bytes(), &jrd)
	require.NoError(t, err)

	var expected api.JRD
	expectedJSON := `{"subject":"acct:example@example.com","aliases":["http://example.com/profile/example"],"properties":{"http://example.com/prop/name":"Example User"},"links":[{"rel":"http://webfinger.net/rel/profile-page","type":"text/html","href":"http://example.com/profile/example"},{"rel":"http://example.com/rel/blog","type":"text/html","href":"http://blogs.example.com/example/"}]}`
	err = json.Unmarshal([]byte(expectedJSON), &expected)
	require.NoError(t, err)

	require.EqualValues(t, expected, jrd)
}
