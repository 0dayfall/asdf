package rest

import (
	"net/http"
	"path"
	"text/template"
)

const templatePath = "template"

var accountTmpl *template.Template
var searchTmpl *template.Template

func init() {
	accountTmpl = template.Must(template.ParseFiles(path.Join(templatePath, "account.html")))
	searchTmpl = template.Must(template.ParseFiles(path.Join(templatePath, "search.html")))
}

func (wfh *WebFingerHandler) HTMLHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		IndexHandler(w, r)
	case http.MethodPost:
		wfh.SearchHandler(w, r)
	case http.MethodPut:
		// Handle PUT request
	case http.MethodPatch:
		// Handle PATCH request
	case http.MethodDelete:
		// Handle DELETE request
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	// Render Go template
	err := searchTmpl.Execute(w, nil)
	if err != nil {
		http.Error(w, "Error rendering template to search", http.StatusInternalServerError)
	}
}

func (wfh *WebFingerHandler) SearchHandler(w http.ResponseWriter, r *http.Request) {
	subject, err := getSubjectFromForm(r)
	if err != nil {
		http.Error(w, "Error parsing form", http.StatusInternalServerError)
	}

	webFingerData, err := wfh.Data.LookupResource(subject)
	if err != nil {
		http.Error(w, "Error lookup resource", http.StatusInternalServerError)
	}

	err = accountTmpl.Execute(w, webFingerData)
	if err != nil {
		http.Error(w, "Error rendering template to display account", http.StatusInternalServerError)
	}
}

func getSubjectFromForm(r *http.Request) (subject string, err error) {
	err = r.ParseForm()
	subject = r.FormValue("acct")
	return
}
