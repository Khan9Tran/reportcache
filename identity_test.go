package reportcache_test

import (
	"errors"
	"testing"
	"time"

	"github.com/Khan9/reportcache"
)

func TestIdentityValidation(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	iv, _ := reportcache.NewInterval(from, to)

	baseID := reportcache.Identity{
		Namespace:    "company-123",
		ReportID:     "daily-revenue",
		ScopeKey:     "offices=001,002",
		BucketID:     "day:UTC",
		Interval:     iv,
		QueryVersion: "v1",
	}

	if err := baseID.Validate(); err != nil {
		t.Fatalf("baseID should be valid, got: %v", err)
	}

	tests := []struct {
		name    string
		mutate  func(id *reportcache.Identity)
		wantErr error
	}{
		{
			name:    "empty namespace",
			mutate:  func(id *reportcache.Identity) { id.Namespace = "" },
			wantErr: reportcache.ErrEmptyNamespace,
		},
		{
			name:    "whitespace namespace",
			mutate:  func(id *reportcache.Identity) { id.Namespace = "   " },
			wantErr: reportcache.ErrEmptyNamespace,
		},
		{
			name:    "empty report ID",
			mutate:  func(id *reportcache.Identity) { id.ReportID = "" },
			wantErr: reportcache.ErrEmptyReportID,
		},
		{
			name:    "empty scope key",
			mutate:  func(id *reportcache.Identity) { id.ScopeKey = "" },
			wantErr: reportcache.ErrEmptyScopeKey,
		},
		{
			name:    "empty bucket ID",
			mutate:  func(id *reportcache.Identity) { id.BucketID = "" },
			wantErr: reportcache.ErrNilBucketSpec,
		},
		{
			name:    "empty query version",
			mutate:  func(id *reportcache.Identity) { id.QueryVersion = "" },
			wantErr: reportcache.ErrEmptyQueryVersion,
		},
		{
			name:    "invalid interval",
			mutate:  func(id *reportcache.Identity) { id.Interval = reportcache.Interval{} },
			wantErr: reportcache.ErrInvalidInterval,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			idCopy := baseID
			tt.mutate(&idCopy)
			err := idCopy.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestIdentityStorageKeyAndCollisionResistance(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	iv, _ := reportcache.NewInterval(from, to)

	id1 := reportcache.Identity{
		Namespace:    "ns:a",
		ReportID:     "rep",
		ScopeKey:     "scope",
		BucketID:     "b",
		Interval:     iv,
		QueryVersion: "v1",
	}

	id2 := reportcache.Identity{
		Namespace:    "ns",
		ReportID:     "a:rep",
		ScopeKey:     "scope",
		BucketID:     "b",
		Interval:     iv,
		QueryVersion: "v1",
	}

	// Delimiter collision test:
	// A naive concatenation "ns:a" + ":" + "rep" == "ns" + ":" + "a:rep" -> "ns:a:rep".
	// But length-prefixed encoding must guarantee id1 and id2 produce DIFFERENT storage keys!
	key1 := id1.StorageKey()
	key2 := id2.StorageKey()

	if key1 == key2 {
		t.Fatalf("StorageKey collision detected! key1=%q, key2=%q", key1, key2)
	}

	if id1.Digest() == id2.Digest() {
		t.Fatalf("Digest collision detected! digest1=%s, digest2=%s", id1.Digest(), id2.Digest())
	}

	// Invariance test: same inputs must produce exact same key and digest
	id1Copy := id1
	if id1Copy.StorageKey() != key1 || id1Copy.Digest() != id1.Digest() {
		t.Errorf("Identity must be deterministic and invariant")
	}
}
