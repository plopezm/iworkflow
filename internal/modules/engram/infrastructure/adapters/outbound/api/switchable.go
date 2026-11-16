package api

import (
	"context"
	"fmt"
	"sync/atomic"

	engramerrors "iworkflow/internal/modules/engram/domain/errors"
	"iworkflow/internal/modules/engram/domain/models"
	"iworkflow/internal/modules/engram/domain/ports/outbound"
)

// SwitchableStore is a MemoryStore whose Engram address can change while the server runs. Use cases keep one
// instance; Switch replaces the client behind it. Without a client every read is unavailable.
type SwitchableStore struct {
	current atomic.Pointer[Client]
}

var _ outbound.MemoryStore = (*SwitchableStore)(nil)

// NewSwitchableStore builds a store with no client yet.
func NewSwitchableStore() *SwitchableStore {
	return &SwitchableStore{}
}

// Switch points the store at the Engram API at baseURL.
func (s *SwitchableStore) Switch(baseURL string) error {
	client, err := NewClient(baseURL)
	if err != nil {
		return fmt.Errorf("switch engram address: %w", err)
	}
	s.current.Store(client)
	return nil
}

func (s *SwitchableStore) client() (*Client, error) {
	client := s.current.Load()
	if client == nil {
		return nil, fmt.Errorf("no engram address: %w", engramerrors.ErrUnavailable)
	}
	return client, nil
}

// Projects lists the Engram projects.
func (s *SwitchableStore) Projects(ctx context.Context) ([]models.MemoryProject, error) {
	client, err := s.client()
	if err != nil {
		return nil, err
	}
	return client.Projects(ctx)
}

// Recent lists up to limit observations of a project.
func (s *SwitchableStore) Recent(ctx context.Context, project string, limit int) ([]models.Memory, error) {
	client, err := s.client()
	if err != nil {
		return nil, err
	}
	return client.Recent(ctx, project, limit)
}

// Observation reads one observation with its content.
func (s *SwitchableStore) Observation(ctx context.Context, id int) (models.MemoryDetail, error) {
	client, err := s.client()
	if err != nil {
		return models.MemoryDetail{}, err
	}
	return client.Observation(ctx, id)
}
