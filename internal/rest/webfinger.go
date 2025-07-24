package rest

import (
	"asdf/internal/resource"
	"asdf/internal/store"
	"asdf/internal/types"
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
	Data store.Store
}

func (wfh *WebFingerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	acct, err := resource.ParseResource(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	jrd, err := wfh.Data.LookupBySubject(r.Context(), acct)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	writeResponse(w, jrd)
}

func writeResponse(w http.ResponseWriter, content *types.JRD) {
	w.Header().Set(ContentType, ContentTypeJRD)

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
