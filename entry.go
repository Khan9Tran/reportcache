package reportcache

import (
	"time"
)

// EntryMetadata contains validity and lifecycle information about a cache entry.
type EntryMetadata struct {
	Identity      Identity   `json:"identity"`
	Generation    int64      `json:"generation"`
	CreatedAt     time.Time  `json:"created_at"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	SourceVersion string     `json:"source_version,omitempty"`
	Completed     bool       `json:"completed"`
}

// CacheEntry wraps the stored payload with its associated validity metadata.
type CacheEntry[T any] struct {
	Metadata EntryMetadata `json:"metadata"`
	Payload  T             `json:"payload"`
}
