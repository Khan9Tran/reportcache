package reportcache_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Khan9Tran/reportcache"
)

type dummyPayload struct {
	TotalRevenue int64
}

func TestPlannerDeterministicPartition(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
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
		ScopeKey:     "currency=VND;offices=001",
		QueryVersion: "v1",
		Interval:     iv,
		Bucket:       bucket,
	}

	// Day 1 (Nov 1): Cached and valid (generation 1)
	// Day 2 (Nov 2): Missing
	// Day 3 (Nov 3): Cached but invalidated (generation 0 vs current report generation 1)
	cacheMap := make(map[string]*reportcache.CacheEntry[dummyPayload])

	day1Start := time.Date(2026, 11, 1, 0, 0, 0, 0, loc)
	day1End := time.Date(2026, 11, 2, 0, 0, 0, 0, loc)
	day1Iv, _ := reportcache.NewInterval(day1Start, day1End)
	day1ID := reportcache.Identity{
		Namespace:    req.Namespace,
		ReportID:     req.ReportID,
		ScopeKey:     req.ScopeKey,
		BucketID:     bucket.ID(),
		Interval:     day1Iv,
		QueryVersion: req.QueryVersion,
	}
	cacheMap[day1ID.StorageKey()] = &reportcache.CacheEntry[dummyPayload]{
		Metadata: reportcache.EntryMetadata{
			Identity:   day1ID,
			Generation: 1,
			Completed:  true,
		},
		Payload: dummyPayload{TotalRevenue: 100},
	}

	day3Start := time.Date(2026, 11, 3, 0, 0, 0, 0, loc)
	day3End := time.Date(2026, 11, 4, 0, 0, 0, 0, loc)
	day3Iv, _ := reportcache.NewInterval(day3Start, day3End)
	day3ID := reportcache.Identity{
		Namespace:    req.Namespace,
		ReportID:     req.ReportID,
		ScopeKey:     req.ScopeKey,
		BucketID:     bucket.ID(),
		Interval:     day3Iv,
		QueryVersion: req.QueryVersion,
	}
	cacheMap[day3ID.StorageKey()] = &reportcache.CacheEntry[dummyPayload]{
		Metadata: reportcache.EntryMetadata{
			Identity:   day3ID,
			Generation: 0, // Stale generation
			Completed:  true,
		},
		Payload: dummyPayload{TotalRevenue: 300},
	}

	mockProvider := func(ctx context.Context, id reportcache.Identity) (*reportcache.CacheEntry[dummyPayload], error) {
		return cacheMap[id.StorageKey()], nil
	}

	planner := reportcache.NewPlanner[dummyPayload](nil)
	plan, err := planner.Plan(ctx, req, mockProvider, reportcache.ValidityContext{
		ReportGeneration: 1,
	})
	if err != nil {
		t.Fatalf("Plan() failed: %v", err)
	}

	if len(plan.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(plan.Items))
	}
	if len(plan.Hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(plan.Hits))
	}
	if len(plan.Fetches) != 2 {
		t.Fatalf("expected 2 fetches, got %d", len(plan.Fetches))
	}

	// Verify hit on Day 1
	if plan.Hits[0].State != reportcache.StateValid {
		t.Errorf("Hit state = %v, want StateValid", plan.Hits[0].State)
	}
	if plan.Hits[0].Entry.Payload.TotalRevenue != 100 {
		t.Errorf("Hit payload = %d, want 100", plan.Hits[0].Entry.Payload.TotalRevenue)
	}

	// Verify fetches: Day 2 is StateMissing, Day 3 is StateInvalidated
	if plan.Fetches[0].State != reportcache.StateMissing {
		t.Errorf("Fetches[0] state = %v, want StateMissing", plan.Fetches[0].State)
	}
	if plan.Fetches[1].State != reportcache.StateInvalidated {
		t.Errorf("Fetches[1] state = %v, want StateInvalidated", plan.Fetches[1].State)
	}
}

func TestPlannerInvalidRequest(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	planner := reportcache.NewPlanner[dummyPayload](nil)

	// Missing namespace
	req := reportcache.Request{
		ReportID: "rev",
	}
	_, err := planner.Plan(ctx, req, nil, reportcache.ValidityContext{})
	if err == nil {
		t.Errorf("expected error for empty namespace")
	}
	if !errors.Is(err, reportcache.ErrNilBucketSpec) {
		t.Errorf("expected ErrNilBucketSpec, got %v", err)
	}
}
