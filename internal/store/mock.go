package store

import (
	"asdf/internal/types"
	"context"
)

type MockStore struct {
	Records map[string]*types.JRD
}

func NewMockStore() *MockStore {
	return &MockStore{
		Records: map[string]*types.JRD{},
	}
}

func (m *MockStore) Add(jrd *types.JRD) {
	m.Records[jrd.Subject] = jrd
}

func (m *MockStore) LookupBySubject(_ context.Context, subject string) (*types.JRD, error) {
	if val, ok := m.Records[subject]; ok {
		return val, nil
	}
	return nil, nil
}
