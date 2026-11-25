package reportcache_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Khan9Tran/reportcache"
	"github.com/Khan9Tran/reportcache/adapters/memory"
)

type RevenuePayload struct {
	Amount int64
}

func TestResolveEndToEnd(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New[RevenuePayload](nil)
	loc := time.UTC
	bucket, err := reportcache.NewCalendarDayBucket(loc)
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	from := time.Date(2026, 11, 1, 0, 0, 0, 0, loc)
	to := time.Date(2026, 11, 4, 0, 0, 0, 0, loc)
	iv, _ := reportcache.NewInterval(from, to)

	req := reportcache.Request{
		Namespace:    "company-123",
		ReportID:     "daily-revenue",
		ScopeKey:     "currency=VND;offices=001,002",
		QueryVersion: "v1",
		Interval:     iv,
		Bucket:       bucket,
	}

	var fetchCalls int64
	fetch := func(ctx context.Context, id reportcache.Identity) (RevenuePayload, error) {
		atomic.AddInt64(&fetchCalls, 1)
		// Return amount proportional to day of month
		return RevenuePayload{Amount: int64(id.Interval.From.Day() * 100)}, nil
	}

	// First Resolve: Cache is completely empty -> 3 fetches
	res1, err := reportcache.Resolve(ctx, req, store, fetch)
	if err != nil {
		t.Fatalf("Resolve 1 failed: %v", err)
	}

	if len(res1.Buckets) != 3 {
		t.Fatalf("expected 3 bucket results, got %d", len(res1.Buckets))
	}
	if atomic.LoadInt64(&fetchCalls) != 3 {
		t.Fatalf("expected 3 fetch calls, got %d", atomic.LoadInt64(&fetchCalls))
	}
	for i, b := range res1.Buckets {
		if b.FromCache {
			t.Errorf("Bucket %d should NOT be from cache on first run", i)
		}
		expectedAmount := int64((1 + i) * 100)
		if b.Payload.Amount != expectedAmount {
			t.Errorf("Bucket %d amount = %d, want %d", i, b.Payload.Amount, expectedAmount)
		}
	}

	// Second Resolve: Exact same request -> 100% cache hits, 0 additional fetch calls!
	res2, err := reportcache.Resolve(ctx, req, store, fetch)
	if err != nil {
		t.Fatalf("Resolve 2 failed: %v", err)
	}

	if len(res2.Buckets) != 3 {
		t.Fatalf("expected 3 bucket results, got %d", len(res2.Buckets))
	}
	if atomic.LoadInt64(&fetchCalls) != 3 {
		t.Errorf("expected still 3 fetch calls (all cache hits), got %d", atomic.LoadInt64(&fetchCalls))
	}
	for i, b := range res2.Buckets {
		if !b.FromCache {
			t.Errorf("Bucket %d should be FROM CACHE on second run", i)
		}
	}

	// Third Resolve with Strategy B invalidation: Bump ReportGeneration from 1 to 2
	// All existing entries become invalidated -> all 3 buckets re-fetched!
	res3, err := reportcache.Resolve(ctx, req, store, fetch,
		reportcache.WithGeneration(2),
		reportcache.WithReportGeneration(2),
		reportcache.WithMaxConcurrency(2),
		reportcache.WithScopeGeneration(1),
		reportcache.WithValidityPolicy(reportcache.NewGenerationPolicy()),
	)
	if err != nil {
		t.Fatalf("Resolve 3 failed: %v", err)
	}

	if atomic.LoadInt64(&fetchCalls) != 6 {
		t.Errorf("expected 6 fetch calls after report generation bump, got %d", atomic.LoadInt64(&fetchCalls))
	}
	for i, b := range res3.Buckets {
		if b.FromCache {
			t.Errorf("Bucket %d should be refreshed (not from cache) after generation bump", i)
		}
	}

	// Fourth Resolve with TTL expiration
	currentTime := time.Now()
	clock := func() time.Time { return currentTime }

	ttlStore := memory.New[RevenuePayload](clock)
	var ttlFetches int64
	ttlFetch := func(ctx context.Context, id reportcache.Identity) (RevenuePayload, error) {
		atomic.AddInt64(&ttlFetches, 1)
		return RevenuePayload{Amount: 100}, nil
	}

	// Run with 1-minute TTL
	_, _ = reportcache.Resolve(ctx, req, ttlStore, ttlFetch,
		reportcache.WithTTL(time.Minute),
		reportcache.WithClock(clock),
	)
	if atomic.LoadInt64(&ttlFetches) != 3 {
		t.Fatalf("expected 3 fetches on first TTL run, got %d", atomic.LoadInt64(&ttlFetches))
	}

	// Advance time by 2 minutes -> Cache is now Stale
	currentTime = currentTime.Add(2 * time.Minute)
	_, _ = reportcache.Resolve(ctx, req, ttlStore, ttlFetch,
		reportcache.WithTTL(time.Minute),
		reportcache.WithClock(clock),
	)
	if atomic.LoadInt64(&ttlFetches) != 6 {
		t.Fatalf("expected 6 fetches after TTL expiration, got %d", atomic.LoadInt64(&ttlFetches))
	}
}

func TestResolveScopeIsolation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New[RevenuePayload](nil)
	loc := time.UTC
	bucket, _ := reportcache.NewCalendarDayBucket(loc)

	from := time.Date(2026, 11, 1, 0, 0, 0, 0, loc)
	to := time.Date(2026, 11, 2, 0, 0, 0, 0, loc)
	iv, _ := reportcache.NewInterval(from, to)

	reqOffice1 := reportcache.Request{
		Namespace:    "company-123",
		ReportID:     "revenue",
		ScopeKey:     "offices=001",
		QueryVersion: "v1",
		Interval:     iv,
		Bucket:       bucket,
	}

	reqOffice2 := reportcache.Request{
		Namespace:    "company-123",
		ReportID:     "revenue",
		ScopeKey:     "offices=002",
		QueryVersion: "v1",
		Interval:     iv,
		Bucket:       bucket,
	}

	var fetchCalls int64
	fetch := func(ctx context.Context, id reportcache.Identity) (RevenuePayload, error) {
		atomic.AddInt64(&fetchCalls, 1)
		if id.ScopeKey == "offices=001" {
			return RevenuePayload{Amount: 500}, nil
		}
		return RevenuePayload{Amount: 900}, nil
	}

	res1, _ := reportcache.Resolve(ctx, reqOffice1, store, fetch)
	if res1.Buckets[0].Payload.Amount != 500 {
		t.Errorf("Office 1 amount = %d, want 500", res1.Buckets[0].Payload.Amount)
	}

	// Office 2 query must NOT hit Office 1 cache!
	res2, _ := reportcache.Resolve(ctx, reqOffice2, store, fetch)
	if res2.Buckets[0].Payload.Amount != 900 {
		t.Errorf("Office 2 amount = %d, want 900", res2.Buckets[0].Payload.Amount)
	}
	if res2.Buckets[0].FromCache {
		t.Errorf("Office 2 must be a miss and not hit Office 1 cache!")
	}
	if atomic.LoadInt64(&fetchCalls) != 2 {
		t.Errorf("fetch calls = %d, want 2 (1 per scope)", atomic.LoadInt64(&fetchCalls))
	}
}

func TestResolveFetchError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New[RevenuePayload](nil)
	loc := time.UTC
	bucket, _ := reportcache.NewCalendarDayBucket(loc)

	from := time.Date(2026, 11, 1, 0, 0, 0, 0, loc)
	to := time.Date(2026, 11, 2, 0, 0, 0, 0, loc)
	iv, _ := reportcache.NewInterval(from, to)

	req := reportcache.Request{
		Namespace:    "company-123",
		ReportID:     "revenue",
		ScopeKey:     "offices=001",
		QueryVersion: "v1",
		Interval:     iv,
		Bucket:       bucket,
	}

	expectedErr := errors.New("upstream database connection timeout")
	fetchFail := func(ctx context.Context, id reportcache.Identity) (RevenuePayload, error) {
		return RevenuePayload{}, expectedErr
	}

	_, err := reportcache.Resolve(ctx, req, store, fetchFail)
	if err == nil {
		t.Fatalf("expected error on failed fetch")
	}

	var bucketErr reportcache.BucketError
	if !errors.As(err, &bucketErr) {
		t.Fatalf("expected error to wrap BucketError, got: %v", err)
	}
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error to unwrap to expectedErr, got: %v", err)
	}

	// Storage must still be empty! Incomplete or failed fetch must never be marked complete.
	if store.Count() != 0 {
		t.Errorf("failed fetch must never write completed entry to storage; store count = %d", store.Count())
	}
}
