package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/0dayfall/webfinger-server/internal/store"
	"github.com/0dayfall/webfinger-server/internal/types"

	"github.com/stretchr/testify/require"
)

func exampleJRD() *types.JRD {
	return &types.JRD{
		Subject: "acct:example@example.com",
		Aliases: []string{"http://example.com/profile/example"},
		Properties: map[string]interface{}{
			"http://example.com/prop/name": "Example User",
		},
		Links: []types.Link{
			{Rel: "http://webfinger.net/rel/profile-page", Type: "text/html", Href: "http://example.com/profile/example"},
			{Rel: "http://example.com/rel/blog", Type: "text/html", Href: "http://blogs.example.com/example/"},
		},
	}
}

func handlerWithExample() *WebFingerHandler {
	mockStore := store.NewMockStore()
	mockStore.Add(exampleJRD())
	return &WebFingerHandler{Data: mockStore}
}

func get(t *testing.T, wfh *WebFingerHandler, target string) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, target, nil)
	require.NoError(t, err)
	wfh.ServeHTTP(rr, request)
	return rr
}

func TestRejectsMissingResource(t *testing.T) {
	rr := get(t, handlerWithExample(), "/.well-known/webfinger")
	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Contains(t, rr.Body.String(), "missing resource")
}

func TestRejectsInvalidResource(t *testing.T) {
	rr := get(t, handlerWithExample(), "/.well-known/webfinger?resource=acct:")
	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Contains(t, rr.Body.String(), "invalid resource")
}

func TestReturnsKnownUser(t *testing.T) {
	rr := get(t, handlerWithExample(), "/.well-known/webfinger?resource=acct:example@example.com")

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, ContentTypeJRD, rr.Header().Get("Content-Type"))
	require.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"),
		"RFC 7033 requires cross-origin access on the WebFinger endpoint")

	var jrd types.JRD
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&jrd))
	require.Equal(t, "acct:example@example.com", jrd.Subject)
	require.Equal(t, "Example User", jrd.Properties["http://example.com/prop/name"])
	require.Len(t, jrd.Links, 2)
}

func TestUnknownUserIs404(t *testing.T) {
	rr := get(t, handlerWithExample(), "/.well-known/webfinger?resource=acct:ghost@example.com")
	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestRelFilteringReturnsOnlyRequestedLinks(t *testing.T) {
	rel := url.QueryEscape("http://webfinger.net/rel/profile-page")
	rr := get(t, handlerWithExample(),
		"/.well-known/webfinger?resource=acct:example@example.com&rel="+rel)

	require.Equal(t, http.StatusOK, rr.Code)

	var jrd types.JRD
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&jrd))
	require.Len(t, jrd.Links, 1)
	require.Equal(t, "http://webfinger.net/rel/profile-page", jrd.Links[0].Rel)
	require.Equal(t, "acct:example@example.com", jrd.Subject, "subject survives filtering")
}

func TestRelFilteringWithNoMatchesReturnsEmptyLinks(t *testing.T) {
	rr := get(t, handlerWithExample(),
		"/.well-known/webfinger?resource=acct:example@example.com&rel=http://example.com/rel/unknown")

	require.Equal(t, http.StatusOK, rr.Code)

	var jrd types.JRD
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&jrd))
	require.Empty(t, jrd.Links)
}

func TestBareAddressLookupResolvesToAcct(t *testing.T) {
	rr := get(t, handlerWithExample(), "/.well-known/webfinger?resource=example@example.com")
	require.Equal(t, http.StatusOK, rr.Code)
}

func TestPostIsMethodNotAllowed(t *testing.T) {
	rr := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost,
		"/.well-known/webfinger?resource=acct:example@example.com", nil)
	require.NoError(t, err)

	handlerWithExample().ServeHTTP(rr, request)
	require.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestOptionsPreflightIsAllowed(t *testing.T) {
	rr := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodOptions, "/.well-known/webfinger", nil)
	require.NoError(t, err)

	handlerWithExample().ServeHTTP(rr, request)
	require.Equal(t, http.StatusNoContent, rr.Code)
	require.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
}
