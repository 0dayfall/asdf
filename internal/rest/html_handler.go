package rest

import (
	"asdf/internal/store"
	"html/template"
	"net/http"
	"path"
)

var templatePath = path.Join("web", "template")

var accountTmpl *template.Template
var searchTmpl *template.Template

// LoadTemplates must be called once during startup.
func LoadTemplates() {
	searchTmpl = template.Must(template.ParseFiles(path.Join(templatePath, "search.html")))
	accountTmpl = template.Must(template.ParseFiles(path.Join(templatePath, "account.html")))
}

// HTMLHandler handles rendering HTML pages for WebFinger search and result display.
type HTMLHandler struct {
	Data store.Store
}

// ServeHTTP routes method-specific handlers for HTML UI.
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

// IndexHandler renders the search form page.
func (h *HTMLHandler) IndexHandler(w http.ResponseWriter, r *http.Request) {
	err := searchTmpl.Execute(w, nil)
	if err != nil {
		http.Error(w, "Error rendering search form", http.StatusInternalServerError)
	}
}

// SearchHandler handles form submission and displays the resolved WebFinger account.
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
