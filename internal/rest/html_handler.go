package rest

import (
	"context"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/0dayfall/webfinger-server/internal/auth"
	"github.com/0dayfall/webfinger-server/internal/store"
	"github.com/0dayfall/webfinger-server/internal/types"
)

var templatePath = path.Join("web", "template")

var (
	accountTmpl     *template.Template
	searchTmpl      *template.Template
	loginTmpl       *template.Template
	registerTmpl    *template.Template
	dashboardTmpl   *template.Template
	connectionsTmpl *template.Template
)

func LoadTemplates() {
	searchTmpl      = template.Must(template.ParseFiles(path.Join(templatePath, "search.html")))
	accountTmpl     = template.Must(template.ParseFiles(path.Join(templatePath, "account.html")))
	loginTmpl       = template.Must(template.ParseFiles(path.Join(templatePath, "login.html")))
	registerTmpl    = template.Must(template.ParseFiles(path.Join(templatePath, "register.html")))
	dashboardTmpl   = template.Must(template.ParseFiles(path.Join(templatePath, "dashboard.html")))
	connectionsTmpl = template.Must(template.ParseFiles(path.Join(templatePath, "connections.html")))
}

type cacheIface interface {
	GetWebFingerRecord(ctx context.Context, subject string) (*types.JRD, error)
	SetWebFingerRecord(ctx context.Context, subject string, jrd *types.JRD, expiry time.Duration) error
	RecordMiss(ctx context.Context)
}

type HTMLHandler struct {
	Data    store.Store
	Cache   cacheIface
	Links   store.LinkStore
	Conns   store.ConnectionStore
	Users   auth.UserStore
	AuthSvc *auth.AuthService
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
	if err := searchTmpl.Execute(w, nil); err != nil {
		log.Printf("Template execution failed: %v", err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
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

	if err := accountTmpl.Execute(w, result); err != nil {
		http.Error(w, "Error rendering account template", http.StatusInternalServerError)
	}
}

func (h *HTMLHandler) LoginPageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := loginTmpl.Execute(w, nil); err != nil {
		log.Printf("Login template error: %v", err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
	}
}

func (h *HTMLHandler) RegisterPageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := registerTmpl.Execute(w, nil); err != nil {
		log.Printf("Register template error: %v", err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
	}
}

func (h *HTMLHandler) DashboardPageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := dashboardTmpl.Execute(w, nil); err != nil {
		log.Printf("Dashboard template error: %v", err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
	}
}

func (h *HTMLHandler) ConnectionsPageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := connectionsTmpl.Execute(w, nil); err != nil {
		log.Printf("Connections template error: %v", err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
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
		w.Header().Set("Content-Type", "application/json")
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
	if err := json.NewEncoder(w).Encode(map[string][]string{"results": results}); err != nil {
		log.Printf("JSON encode failed: %v", err)
	}
}

// HandleWebFinger serves RFC 7033 responses with visibility filtering.
// Public callers see only public links; authenticated users get registered-level;
// friends see friend-level links.
func (h *HTMLHandler) HandleWebFinger(w http.ResponseWriter, r *http.Request) {
	resource := r.URL.Query().Get("resource")
	if resource == "" {
		http.Error(w, "missing resource param", http.StatusBadRequest)
		return
	}

	// Resolve viewer identity from optional JWT.
	var viewerID *int
	if h.AuthSvc != nil {
		if tok := bearerToken(r); tok != "" {
			if claims, err := h.AuthSvc.ValidateToken(r.Context(), tok); err == nil {
				viewerID = &claims.UserID
			}
		}
	}

	// For cached responses we only serve public (viewer==nil) to keep things simple.
	if viewerID == nil && h.Cache != nil {
		resp, err := h.Cache.GetWebFingerRecord(r.Context(), resource)
		if err == nil && resp != nil {
			w.Header().Set("Content-Type", "application/jrd+json")
			w.Header().Set("X-Cache", "HIT")
			filterByRel(resp, r.URL.Query()["rel"])
			json.NewEncoder(w).Encode(resp)
			return
		}
		h.Cache.RecordMiss(r.Context())
	}

	// Look up subject JRD (for aliases/properties).
	jrd, err := h.Data.LookupBySubject(r.Context(), resource)
	if err != nil || jrd == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	resp := &types.JRD{
		Subject:    jrd.Subject,
		Aliases:    jrd.Aliases,
		Properties: jrd.Properties,
		Links:      jrd.Links,
	}

	// Override links with visibility-filtered set when stores are wired up.
	if h.Links != nil && h.Users != nil {
		subjectUser, err := h.Users.GetUserBySubject(r.Context(), resource)
		if err == nil && subjectUser != nil {
			areFriends := false
			if viewerID != nil && h.Conns != nil {
				areFriends, _ = h.Conns.AreFriends(r.Context(), subjectUser.ID, *viewerID)
			}
			if userLinks, err := h.Links.GetLinksForViewer(r.Context(), subjectUser.ID, viewerID, areFriends); err == nil {
				resp.Links = userLinksToJRD(userLinks)
			}
		}
	}

	// Cache public view.
	if viewerID == nil && h.Cache != nil {
		_ = h.Cache.SetWebFingerRecord(r.Context(), resource, resp, 5*time.Minute)
	}

	filterByRel(resp, r.URL.Query()["rel"])

	w.Header().Set("Content-Type", "application/jrd+json")
	w.Header().Set("X-Cache", "MISS")
	json.NewEncoder(w).Encode(resp)
}

// filterByRel removes links whose Rel is not in the requested set.
func filterByRel(jrd *types.JRD, rels []string) {
	if len(rels) == 0 || jrd == nil {
		return
	}
	want := make(map[string]struct{}, len(rels))
	for _, r := range rels {
		want[r] = struct{}{}
	}
	filtered := make([]types.Link, 0, len(jrd.Links))
	for _, link := range jrd.Links {
		if _, ok := want[link.Rel]; ok {
			filtered = append(filtered, link)
		}
	}
	jrd.Links = filtered
}

func userLinksToJRD(links []store.UserLink) []types.Link {
	out := make([]types.Link, 0, len(links))
	for _, l := range links {
		out = append(out, types.Link{
			Rel:  l.Rel,
			Href: l.Href,
			Type: l.Platform,
		})
	}
	return out
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if after, ok := strings.CutPrefix(h, "Bearer "); ok {
		return after
	}
	return ""
}
