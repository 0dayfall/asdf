// Package resource parses and validates WebFinger resource identifiers
// (RFC 7033). A resource is a URI: typically an acct: URI such as
// acct:alice@example.com, but any absolute URI (e.g. https://example.com/user)
// is permitted by the spec.
package resource

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
)

// ParseResource extracts and canonicalizes the ?resource= query parameter.
func ParseResource(request *http.Request) (string, error) {
	res := request.URL.Query().Get("resource")
	if res == "" {
		return "", errors.New("webfinger-server: missing resource parameter")
	}
	return Canonicalize(res)
}

// Canonicalize validates a WebFinger resource and returns its canonical form.
//
//   - "acct:alice@example.com" stays as-is (validated).
//   - "alice@example.com" (bare address) becomes "acct:alice@example.com".
//   - Any other absolute URI ("https://example.com/user") stays as-is.
func Canonicalize(res string) (string, error) {
	res = strings.TrimSpace(res)
	if res == "" {
		return "", errors.New("webfinger-server: missing resource parameter")
	}

	if rest, ok := strings.CutPrefix(res, "acct:"); ok {
		if !isValidAcct(rest) {
			return "", errors.New("webfinger-server: invalid resource parameter")
		}
		return "acct:" + rest, nil
	}

	// Absolute URI (https://..., mailto:..., etc.)
	if parsed, err := url.Parse(res); err == nil && parsed.Scheme != "" {
		if parsed.Host == "" && parsed.Opaque == "" {
			return "", errors.New("webfinger-server: invalid resource parameter")
		}
		return res, nil
	}

	// Bare user@host is accepted for convenience and canonicalized to acct:.
	if isValidAcct(res) {
		return "acct:" + res, nil
	}

	return "", errors.New("webfinger-server: invalid resource parameter")
}

func isValidAcct(acct string) bool {
	local, domain, found := strings.Cut(acct, "@")
	if !found || local == "" || domain == "" {
		return false
	}
	if strings.ContainsAny(acct, " \t") {
		return false
	}
	return !strings.Contains(domain, "@")
}
