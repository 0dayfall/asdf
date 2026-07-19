package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/0dayfall/webfinger-server/internal/resource"
	"github.com/0dayfall/webfinger-server/internal/types"
)

// FileStore is a read-only Store backed by a JSON file containing an array
// of JRD records. It is the zero-dependency default: point the server at a
// data file and it serves WebFinger lookups with nothing else running.
type FileStore struct {
	records    []types.JRD
	bySubject  map[string]*types.JRD
	subjectSet []string
}

// NewFileStore loads and indexes the JSON data file at path.
func NewFileStore(path string) (*FileStore, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("store: read data file %q: %w", path, err)
	}

	var records []types.JRD
	if err := json.Unmarshal(raw, &records); err != nil {
		return nil, fmt.Errorf("store: parse data file %q: %w", path, err)
	}

	fs := &FileStore{
		records:   records,
		bySubject: make(map[string]*types.JRD, len(records)),
	}
	for i := range fs.records {
		jrd := &fs.records[i]
		key, err := resource.Canonicalize(jrd.Subject)
		if err != nil {
			return nil, fmt.Errorf("store: invalid subject %q in %q: %w", jrd.Subject, path, err)
		}
		fs.bySubject[key] = jrd
		fs.subjectSet = append(fs.subjectSet, jrd.Subject)

		// Also allow lookup by alias so https://... profile URIs resolve.
		for _, alias := range jrd.Aliases {
			if _, exists := fs.bySubject[alias]; !exists {
				fs.bySubject[alias] = jrd
			}
		}
	}

	return fs, nil
}

// Len reports the number of records loaded.
func (fs *FileStore) Len() int {
	return len(fs.records)
}

// LookupBySubject returns the JRD for a canonical subject or alias,
// or nil when unknown.
func (fs *FileStore) LookupBySubject(_ context.Context, subject string) (*types.JRD, error) {
	if jrd, ok := fs.bySubject[subject]; ok {
		return jrd, nil
	}
	return nil, nil
}

// SearchSubjects returns subjects containing the query, case-insensitively.
func (fs *FileStore) SearchSubjects(_ context.Context, query string) ([]string, error) {
	query = strings.ToLower(query)
	var results []string
	for _, subject := range fs.subjectSet {
		if strings.Contains(strings.ToLower(subject), query) {
			results = append(results, subject)
		}
	}
	return results, nil
}
