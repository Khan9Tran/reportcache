package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Khan9Tran/reportcache"
)

// Store is an in-memory, thread-safe implementation of reportcache.CacheStore[T].
// It is intended for testing, development, and standalone applications without Redis.
type Store[T any] struct {
	mu      sync.RWMutex
	entries map[string]reportcache.CacheEntry[T]
	clock   func() time.Time
}

// New creates a new in-memory cache store.
// If clock is nil, time.Now is used.
func New[T any](clock func() time.Time) *Store[T] {
	if clock == nil {
		clock = time.Now
	}
	return &Store[T]{
		entries: make(map[string]reportcache.CacheEntry[T]),
		clock:   clock,
	}
}

// Get retrieves a cache entry by its Identity.
// It returns nil, nil if the entry does not exist or has expired.
func (s *Store[T]) Get(ctx context.Context, id reportcache.Identity) (*reportcache.CacheEntry[T], error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	key := id.StorageKey()

	s.mu.RLock()
	entry, ok := s.entries[key]
	s.mu.RUnlock()

	if !ok {
		return nil, nil
	}

	// Check TTL expiration if configured
	if entry.Metadata.ExpiresAt != nil && s.clock().After(*entry.Metadata.ExpiresAt) {
		return nil, nil
	}

	return &entry, nil
}

// Set stores a cache entry. It rejects stale overwrites where an older generation
// attempts to overwrite an entry with a higher generation.
func (s *Store[T]) Set(ctx context.Context, entry reportcache.CacheEntry[T]) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := entry.Metadata.Identity.Validate(); err != nil {
		return fmt.Errorf("invalid entry identity: %w", err)
	}

	key := entry.Metadata.Identity.StorageKey()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Stale writer protection: do not let an older generation overwrite a newer generation
	if existing, ok := s.entries[key]; ok {
		if entry.Metadata.Generation < existing.Metadata.Generation {
			// Older generation write rejected
			return nil
		}
	}

	s.entries[key] = entry
	return nil
}

// Count returns the number of active entries currently in the store.
func (s *Store[T]) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}
