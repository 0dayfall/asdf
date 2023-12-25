package resource

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmailResource(t *testing.T) {
	// Arrange
	resource := "adsf@example.com"
	correctURL := "https://example.com/.well-known/webfinger?resource=acct:" + resource
	parsedURL, _ := url.Parse(correctURL)
	httpRequest := http.Request{URL: parsedURL}

	// Act
	resource, err := ParseResource(&httpRequest)

	//Evaluate
	require.NoError(t, err)
	require.Equal(t, "adsf@example.com", resource)
}

func TestXResource(t *testing.T) {
	// Arrange
	resource := "@asdf"
	correctURL := "https://example.com/.well-known/webfinger?resource=acct:" + resource
	parsedURL, _ := url.Parse(correctURL)
	httpRequest := http.Request{URL: parsedURL}

	// Act
	resource, err := ParseResource(&httpRequest)

	//Evaluate
	require.NoError(t, err)
	require.Equal(t, "@asdf", resource)
}
