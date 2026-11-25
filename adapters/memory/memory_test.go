package memory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Khan9/reportcache"
	"github.com/Khan9/reportcache/adapters/memory"
)

func TestMemoryStoreContract(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	store := memory.New[string](clock)

	from := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	iv, _ := reportcache.NewInterval(from, to)

	id := reportcache.Identity{
		Namespace:    "tenant-1",
		ReportID:     "report-a",
		ScopeKey:     "scope-x",
		BucketID:     "day:UTC",
		Interval:     iv,
		QueryVersion: "v1",
	}

	// 1. Get non-existent key
	entry, err := store.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}
	if entry != nil {
		t.Fatalf("expected nil entry for missing key, got %+v", entry)
	}

	// 2. Set and Get
	newEntry := reportcache.CacheEntry[string]{
		Metadata: reportcache.EntryMetadata{
			Identity:   id,
			Generation: 1,
			CreatedAt:  now,
			Completed:  true,
		},
		Payload: "payload-data",
	}

	if err := store.Set(ctx, newEntry); err != nil {
		t.Fatalf("Set() failed: %v", err)
	}

	got, err := store.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	if got == nil || got.Payload != "payload-data" {
		t.Fatalf("Get() payload mismatch: %+v", got)
	}

	// 3. Stale writer protection: generation 0 must NOT overwrite generation 1
	staleEntry := reportcache.CacheEntry[string]{
		Metadata: reportcache.EntryMetadata{
			Identity:   id,
			Generation: 0,
			CreatedAt:  now,
			Completed:  true,
		},
		Payload: "stale-payload",
	}
	if err := store.Set(ctx, staleEntry); err != nil {
		t.Fatalf("Set() stale entry failed: %v", err)
	}

	gotAfterStale, _ := store.Get(ctx, id)
	if gotAfterStale.Payload != "payload-data" {
		t.Errorf("stale writer overwrote newer generation! got: %s", gotAfterStale.Payload)
	}

	// 4. Newer generation 2 DOES overwrite generation 1
	newerEntry := reportcache.CacheEntry[string]{
		Metadata: reportcache.EntryMetadata{
			Identity:   id,
			Generation: 2,
			CreatedAt:  now,
			Completed:  true,
		},
		Payload: "v2-payload",
	}
	if err := store.Set(ctx, newerEntry); err != nil {
		t.Fatalf("Set() newer entry failed: %v", err)
	}

	gotAfterNewer, _ := store.Get(ctx, id)
	if gotAfterNewer.Payload != "v2-payload" {
		t.Errorf("newer generation failed to overwrite! got: %s", gotAfterNewer.Payload)
	}

	// 5. Count active entries
	if count := store.Count(); count != 1 {
		t.Errorf("Count() = %d, want 1", count)
	}

	// 6. Context cancellation on Get
	cancCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.Get(cancCtx, id); !errors.Is(err, context.Canceled) {
		t.Errorf("Get with canceled ctx should return context.Canceled, got: %v", err)
	}
	if err := store.Set(cancCtx, newerEntry); !errors.Is(err, context.Canceled) {
		t.Errorf("Set with canceled ctx should return context.Canceled, got: %v", err)
	}
}
