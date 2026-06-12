package idempotency

import (
	"context"
	"sync"
	"time"

	"github.com/sanchit/order-orchestrator/internal/order"
)

type entry struct {
	order     order.Order
	expiresAt time.Time
}

type MemoryStore struct {
	mu      sync.Mutex
	entries map[string]entry
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{entries: make(map[string]entry)}
}

func (s *MemoryStore) Get(_ context.Context, key string) (*order.Order, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.entries[key]
	if !ok {
		return nil, false
	}
	if time.Now().UTC().After(item.expiresAt) {
		delete(s.entries, key)
		return nil, false
	}
	return &item.order, true
}

func (s *MemoryStore) Set(_ context.Context, key string, response *order.Order, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries[key] = entry{
		order:     *response,
		expiresAt: time.Now().UTC().Add(ttl),
	}
	return nil
}
