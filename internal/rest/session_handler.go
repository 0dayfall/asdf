package rest

import (
	"net/http"

	"github.com/gorilla/sessions"
)

type SessionHandler struct {
	session *sessions.CookieStore
}

func (sh SessionHandler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Add your authentication logic here

	// On successful authentication:
	err := sh.session.SetSessionValue(w, r, "username", "your_username")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (sh SessionHandler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	err := sh.session.ClearSession(w, r)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/login", http.StatusFound)
}
