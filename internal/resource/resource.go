package resource

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
)

func ParseResource(request *http.Request) (string, error) {
	res := request.URL.Query().Get("resource")
	if res == "" {
		return "", errors.New("missing resource parameter")
	}
	return GetSubject(res)
}

// GetSubject normalises a resource URI for store lookup.
// acct: URIs are stripped to bare user@domain form.
// https:// and http:// URIs are returned unchanged.
func GetSubject(resource string) (string, error) {
	if strings.HasPrefix(resource, "https://") || strings.HasPrefix(resource, "http://") {
		if !IsValidResource(resource) {
			return "", errors.New("invalid resource parameter")
		}
		return resource, nil
	}
	acct := strings.TrimPrefix(resource, "acct:")
	if !IsValidResource(acct) {
		return "", errors.New("invalid resource parameter")
	}
	return acct, nil
}

// IsValidResource returns true for acct-style strings (must contain @) and for
// well-formed https:// or http:// URIs (RFC 7033 §3).
func IsValidResource(resource string) bool {
	if strings.HasPrefix(resource, "https://") || strings.HasPrefix(resource, "http://") {
		u, err := url.ParseRequestURI(resource)
		return err == nil && u.Host != ""
	}
	return strings.Contains(resource, "@")
}
