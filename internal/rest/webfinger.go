package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/0dayfall/webfinger-server/internal/resource"
	"github.com/0dayfall/webfinger-server/internal/store"
	"github.com/0dayfall/webfinger-server/internal/types"
)

const (
	ContentType    = "Content-Type"
	ContentTypeJRD = "application/jrd+json"

	cacheTTL = 5 * time.Minute
)

// WebFingerCache is the optional cache used by the WebFinger handler.
type WebFingerCache interface {
	GetWebFingerRecord(ctx context.Context, subject string) (*types.JRD, error)
	SetWebFingerRecord(ctx context.Context, subject string, jrd *types.JRD, expiry time.Duration) error
	RecordMiss(ctx context.Context)
}

// WebFingerHandler serves RFC 7033 WebFinger lookups.
type WebFingerHandler struct {
	Data  store.Store
	Cache WebFingerCache
}

func (wfh *WebFingerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// RFC 7033 requires support for cross-origin requests on this endpoint.
	// This is public, non-credentialed data: a wildcard is correct here.
	w.Header().Set("Access-Control-Allow-Origin", "*")

	switch r.Method {
	case http.MethodOptions:
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
		return
	case http.MethodGet:
		// fall through
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	subject, err := resource.ParseResource(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	jrd, cacheStatus := wfh.lookup(r.Context(), subject)
	if jrd == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// RFC 7033 section 4.3: when one or more rel parameters are present,
	// filter the links to those matching the requested relation types.
	// Filtering copies the JRD so cached/stored records stay intact.
	if rels := r.URL.Query()["rel"]; len(rels) > 0 {
		jrd = filterRels(jrd, rels)
	}

	if cacheStatus != "" {
		w.Header().Set("X-Cache", cacheStatus)
	}
	writeResponse(w, jrd)
}

// lookup fetches the JRD for a subject, consulting the cache when configured.
// It returns nil when the subject is unknown.
func (wfh *WebFingerHandler) lookup(ctx context.Context, subject string) (*types.JRD, string) {
	if wfh.Cache != nil {
		if jrd, err := wfh.Cache.GetWebFingerRecord(ctx, subject); err == nil && jrd != nil {
			return jrd, "HIT"
		}
		wfh.Cache.RecordMiss(ctx)
	}

	jrd, err := wfh.Data.LookupBySubject(ctx, subject)
	if err != nil || jrd == nil {
		return nil, ""
	}

	if wfh.Cache != nil {
		_ = wfh.Cache.SetWebFingerRecord(ctx, subject, jrd, cacheTTL)
		return jrd, "MISS"
	}
	return jrd, ""
}

// filterRels returns a copy of the JRD whose links are limited to the
// requested link relation types.
func filterRels(jrd *types.JRD, rels []string) *types.JRD {
	wanted := make(map[string]struct{}, len(rels))
	for _, rel := range rels {
		wanted[rel] = struct{}{}
	}

	filtered := *jrd
	filtered.Links = make([]types.Link, 0, len(jrd.Links))
	for _, link := range jrd.Links {
		if _, ok := wanted[link.Rel]; ok {
			filtered.Links = append(filtered.Links, link)
		}
	}
	return &filtered
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
