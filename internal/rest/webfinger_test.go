package rest

import (
	"asdf/internal/store"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGETResourceEmpty(t *testing.T) {
	t.Run("Rejects missing resource", func(t *testing.T) {
		mockStore := store.NewMockStore()
		wfh := WebFingerHandler{Data: mockStore}

		rr := httptest.NewRecorder()
		request, err := http.NewRequest(http.MethodGet, "/.well-known/webfinger?resource=acct:", nil)
		require.NoError(t, err)

		// Act
		wfh.ServeHTTP(rr, request)

		// Assert
		require.Equal(t, http.StatusBadRequest, rr.Code)
		require.Contains(t, rr.Body.String(), "invalid resource")
	})
}
