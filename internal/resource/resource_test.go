package resource

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func requestFor(t *testing.T, rawURL string) *http.Request {
	t.Helper()
	parsedURL, err := url.Parse(rawURL)
	require.NoError(t, err)
	return &http.Request{URL: parsedURL}
}

func TestParseAcctResource(t *testing.T) {
	req := requestFor(t, "https://example.com/.well-known/webfinger?resource=acct:alice@example.com")

	got, err := ParseResource(req)

	require.NoError(t, err)
	require.Equal(t, "acct:alice@example.com", got)
}

func TestParseBareAddressCanonicalizesToAcct(t *testing.T) {
	req := requestFor(t, "https://example.com/.well-known/webfinger?resource=alice@example.com")

	got, err := ParseResource(req)

	require.NoError(t, err)
	require.Equal(t, "acct:alice@example.com", got)
}

func TestParseHTTPSResource(t *testing.T) {
	req := requestFor(t, "https://example.com/.well-known/webfinger?resource="+
		url.QueryEscape("https://example.com/users/alice"))

	got, err := ParseResource(req)

	require.NoError(t, err)
	require.Equal(t, "https://example.com/users/alice", got)
}

func TestParseMailtoResource(t *testing.T) {
	got, err := Canonicalize("mailto:bob@example.org")

	require.NoError(t, err)
	require.Equal(t, "mailto:bob@example.org", got)
}

func TestMissingResource(t *testing.T) {
	req := requestFor(t, "https://example.com/.well-known/webfinger")

	_, err := ParseResource(req)

	require.Error(t, err)
	require.Contains(t, err.Error(), "missing resource")
}

func TestInvalidResources(t *testing.T) {
	for _, res := range []string{
		"acct:",            // empty acct
		"acct:@alice",      // no local part
		"acct:alice@",      // no domain
		"acct:a b@example", // whitespace
		"acct:a@b@c",       // double @
		"not a uri",        // no scheme, no @
		"https://",         // scheme without host
	} {
		_, err := Canonicalize(res)
		require.Error(t, err, "expected %q to be rejected", res)
	}
}
