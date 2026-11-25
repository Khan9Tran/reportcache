package reportcache_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Khan9/reportcache"
)

func TestExecutorContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	bucket, _ := reportcache.NewFixedDurationBucket(time.Hour, time.Time{})

	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 1, 5, 0, 0, 0, time.UTC)
	iv, _ := reportcache.NewInterval(from, to)

	req := reportcache.Request{
		Namespace:    "ns",
		ReportID:     "rep",
		ScopeKey:     "scope",
		QueryVersion: "v1",
		Interval:     iv,
		Bucket:       bucket,
	}

	planner := reportcache.NewPlanner[string](nil)
	plan, err := planner.Plan(ctx, req, nil, reportcache.ValidityContext{})
	if err != nil {
		t.Fatalf("Plan() failed: %v", err)
	}

	exec := reportcache.NewExecutor[string](nil, reportcache.ExecutorConfig{
		MaxConcurrency: 1, // serial execution to reliably test early cancellation
	})

	var executedCount int64
	fetch := func(ctx context.Context, id reportcache.Identity) (string, error) {
		count := atomic.AddInt64(&executedCount, 1)
		if count == 1 {
			// cancel context during first fetch
			cancel()
		}
		// check if canceled
		if err := ctx.Err(); err != nil {
			return "", err
		}
		return "data", nil
	}

	_, err = exec.Execute(ctx, plan, fetch)
	if err == nil {
		t.Fatalf("expected error due to cancellation, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestBucketErrorAndValidityStateString(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)
	iv, _ := reportcache.NewInterval(from, to)

	errOrig := errors.New("underlying error")
	bErr := reportcache.BucketError{
		Interval: iv,
		Err:      errOrig,
	}

	if bErr.Error() == "" {
		t.Errorf("Error() should not be empty")
	}
	if !errors.Is(bErr, errOrig) {
		t.Errorf("Unwrap() failed")
	}

	states := []reportcache.ValidityState{
		reportcache.StateValid,
		reportcache.StateMissing,
		reportcache.StateInvalidated,
		reportcache.StateStale,
		reportcache.StateUnknown,
		reportcache.ValidityState(99),
	}

	for _, s := range states {
		if s.String() == "" {
			t.Errorf("String() for state %v should not be empty", s)
		}
	}
}
