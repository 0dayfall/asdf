package rest

import (
	"asdf/internal/store"
	"asdf/internal/types"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"path"
	"strings"
)

var templatePath = path.Join("web", "template")

var accountTmpl *template.Template
var searchTmpl *template.Template

func LoadTemplates() {
	searchTmpl = template.Must(template.ParseFiles(path.Join(templatePath, "search.html")))
	accountTmpl = template.Must(template.ParseFiles(path.Join(templatePath, "account.html")))
}

type HTMLHandler struct {
	Data store.Store
}

func (h *HTMLHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.IndexHandler(w, r)
	case http.MethodPost:
		h.SearchHandler(w, r)
	case http.MethodPut, http.MethodPatch, http.MethodDelete:
		http.Error(w, "Not implemented", http.StatusNotImplemented)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *HTMLHandler) IndexHandler(w http.ResponseWriter, r *http.Request) {
	err := searchTmpl.Execute(w, nil)
	if err != nil {
		log.Printf("Template execution failed: %v", err)
		http.Error(w, "Error rendering search form", http.StatusInternalServerError)
	}

}

func (h *HTMLHandler) SearchHandler(w http.ResponseWriter, r *http.Request) {
	subject, err := getSubjectFromForm(r)
	if err != nil || subject == "" {
		http.Error(w, "Invalid subject", http.StatusBadRequest)
		return
	}

	result, err := h.Data.LookupBySubject(r.Context(), subject)
	if err != nil {
		http.Error(w, "Error during lookup", http.StatusInternalServerError)
		return
	}
	if result == nil {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}

	err = accountTmpl.Execute(w, result)
	if err != nil {
		http.Error(w, "Error rendering account template", http.StatusInternalServerError)
	}
}

func getSubjectFromForm(r *http.Request) (string, error) {
	if err := r.ParseForm(); err != nil {
		return "", err
	}
	return r.FormValue("acct"), nil
}

func (h *HTMLHandler) HandleSearchAPI(w http.ResponseWriter, r *http.Request) {
	query := strings.ToLower(r.URL.Query().Get("q"))
	if len(query) < 2 {
		json.NewEncoder(w).Encode(map[string][]string{"results": {}})
		return
	}

	results, err := h.Data.SearchSubjects(r.Context(), query)
	if err != nil {
		log.Printf("SearchSubjects failed: %v", err)
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(map[string][]string{
		"results": results,
	})
	if err != nil {
		log.Printf("JSON encode failed: %v", err)
	}
}

func (h *HTMLHandler) HandleWebFinger(w http.ResponseWriter, r *http.Request) {
	resource := r.URL.Query().Get("resource")
	if resource == "" {
		http.Error(w, "missing resource param", http.StatusBadRequest)
		return
	}

	user, err := h.Data.LookupBySubject(r.Context(), resource)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	resp := types.JRD{
		Subject:    user.Subject,
		Aliases:    user.Aliases,
		Properties: user.Properties,
		Links:      user.Links,
	}

	w.Header().Set("Content-Type", "application/jrd+json")
	json.NewEncoder(w).Encode(resp)
}
