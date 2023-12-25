package rest

import (
	"asdf/internal/api"
	"asdf/internal/db"
	"asdf/internal/resource"
	"bytes"
	"encoding/json"
	"log"
	"net/http"
)

const (
	ContentType    = "Content-Type"
	ContentTypeJRD = "application/jrd+json"
)

type WebFingerHandler struct {
	Data *db.Data
}

func (wfh *WebFingerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	acct, err := resource.ParseResource(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	jrd, err := wfh.Data.LookupResource(acct)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeResponse(w, jrd)
}

func writeResponse(w http.ResponseWriter, content *api.JRD) {
	w.Header().Set(ContentType, ContentTypeJRD)

	// Use a buffer, should the encoding fail, we don't want to send a partial response
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(content); err != nil {
		log.Printf("Error writing body: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err := buf.WriteTo(w); err != nil {
		log.Printf("Error writing body: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
