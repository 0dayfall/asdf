package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeDataFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

const sampleData = `[
  {
    "subject": "acct:alice@example.com",
    "aliases": ["https://example.com/@alice"],
    "links": [
      {"rel": "http://webfinger.net/rel/profile-page", "type": "text/html", "href": "https://example.com/@alice"}
    ]
  },
  {
    "subject": "acct:bob@example.com",
    "links": []
  }
]`

func TestFileStoreLookup(t *testing.T) {
	fs, err := NewFileStore(writeDataFile(t, sampleData))
	require.NoError(t, err)
	require.Equal(t, 2, fs.Len())

	jrd, err := fs.LookupBySubject(context.Background(), "acct:alice@example.com")
	require.NoError(t, err)
	require.NotNil(t, jrd)
	require.Equal(t, "acct:alice@example.com", jrd.Subject)
	require.Len(t, jrd.Links, 1)
}

func TestFileStoreLookupByAlias(t *testing.T) {
	fs, err := NewFileStore(writeDataFile(t, sampleData))
	require.NoError(t, err)

	jrd, err := fs.LookupBySubject(context.Background(), "https://example.com/@alice")
	require.NoError(t, err)
	require.NotNil(t, jrd)
	require.Equal(t, "acct:alice@example.com", jrd.Subject)
}

func TestFileStoreUnknownSubjectIsNil(t *testing.T) {
	fs, err := NewFileStore(writeDataFile(t, sampleData))
	require.NoError(t, err)

	jrd, err := fs.LookupBySubject(context.Background(), "acct:ghost@example.com")
	require.NoError(t, err)
	require.Nil(t, jrd)
}

func TestFileStoreSearch(t *testing.T) {
	fs, err := NewFileStore(writeDataFile(t, sampleData))
	require.NoError(t, err)

	results, err := fs.SearchSubjects(context.Background(), "ALICE")
	require.NoError(t, err)
	require.Equal(t, []string{"acct:alice@example.com"}, results)
}

func TestFileStoreMissingFile(t *testing.T) {
	_, err := NewFileStore(filepath.Join(t.TempDir(), "nope.json"))
	require.Error(t, err)
}

func TestFileStoreInvalidJSON(t *testing.T) {
	_, err := NewFileStore(writeDataFile(t, "{not json"))
	require.Error(t, err)
}

func TestFileStoreRejectsInvalidSubject(t *testing.T) {
	_, err := NewFileStore(writeDataFile(t, `[{"subject": "acct:"}]`))
	require.Error(t, err)
}

func TestFileStoreLoadsRepositorySampleData(t *testing.T) {
	fs, err := NewFileStore(filepath.Join("..", "..", "data", "data.json"))
	require.NoError(t, err)
	require.GreaterOrEqual(t, fs.Len(), 1)
}
