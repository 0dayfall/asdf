package rest

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"text/template"
	"time"

	"github.com/golang-jwt/jwt"
)

func generateSecretKey(length int) (string, error) {
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

func LoginPage(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		username := r.FormValue("username")
		password := r.FormValue("password")

		if username == "admin" && password == "admin" {
			token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"nbf": time.Now().Unix(),
			})

			hmacSampleSecret, err := generateSecretKey(32)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_, err = token.SignedString(hmacSampleSecret)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			http.Redirect(w, r, "/", http.StatusSeeOther)
		}
		fmt.Fprintf(w, "Invalid credentials. Please try again.")
		return
	}

	// If not a POST request, serve the login page template.
	tmpl, err := template.ParseFiles("templates/login.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}
