package rest

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"

	"github.com/0dayfall/webfinger-server/internal/resource"
	"github.com/0dayfall/webfinger-server/internal/store"
	"github.com/0dayfall/webfinger-server/internal/types"
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

	// Filter links by ?rel= parameters (RFC 7033 §4.4)
	filterByRel(jrd, r.URL.Query()["rel"])

	writeResponse(w, jrd)
}

func writeResponse(w http.ResponseWriter, content *types.JRD) {
	w.Header().Set(ContentType, ContentTypeJRD)

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(content); err != nil {
		log.Printf("Error encoding JRD: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err := buf.WriteTo(w); err != nil {
		log.Printf("Error writing response: %v", err)
	}
}
