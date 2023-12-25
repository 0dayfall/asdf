package resource

import (
	"errors"
	"net/http"
	"strings"
)

func ParseResource(request *http.Request) (string, error) {
	resource := request.URL.Query().Get("resource")
	if resource == "" {
		return "", errors.New("asdf: missing resource parameter")
	}

	acct, err := GetSubject(resource)
	if err != nil {
		return "", err
	}

	return acct, nil
}

func GetSubject(resource string) (string, error) {
	acct := strings.TrimPrefix(resource, "acct:")
	if !IsValidResource(acct) {
		return "", errors.New("asdf: invalid resource parameter")
	}
	return acct, nil
}

func IsValidResource(resource string) bool {
	return strings.Contains(resource, "@")
}
