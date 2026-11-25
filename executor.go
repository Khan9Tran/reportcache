package reportcache

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// BucketError represents a failure encountered while fetching a specific bucket interval.
type BucketError struct {
	Interval Interval
	Err      error
}

func (e BucketError) Error() string {
	return fmt.Sprintf("reportcache: fetch failed for bucket %s: %v", e.Interval, e.Err)
}

func (e BucketError) Unwrap() error {
	return e.Err
}

// FetchFunc is the application-provided callback to load data for a specific bucket and scope.
type FetchFunc[T any] func(ctx context.Context, id Identity) (T, error)

// ExecutorConfig holds concurrency and lifecycle settings for executing fetches.
type ExecutorConfig struct {
	MaxConcurrency int
	Generation     int64
	TTL            time.Duration
	Clock          func() time.Time
}

// DefaultExecutorConfig returns conservative, safe defaults for executor configuration.
func DefaultExecutorConfig() ExecutorConfig {
	return ExecutorConfig{
		MaxConcurrency: 4,
		Generation:     1,
		Clock:          time.Now,
	}
}

// Executor coordinates bounded-concurrency execution of missing and invalid buckets.
type Executor[T any] struct {
	cfg   ExecutorConfig
	store CacheStore[T]
}

// NewExecutor creates a new Executor with the given storage and configuration.
func NewExecutor[T any](store CacheStore[T], cfg ExecutorConfig) *Executor[T] {
	if cfg.MaxConcurrency <= 0 {
		cfg.MaxConcurrency = 4
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	return &Executor[T]{
		store: store,
		cfg:   cfg,
	}
}

// Execute executes all required fetches in plan with bounded concurrency,
// writes successful results back to storage, and returns all bucket results in chronological order.
func (e *Executor[T]) Execute(
	ctx context.Context,
	plan *Plan[T],
	fetch FetchFunc[T],
) (*Result[T], error) {
	if plan == nil {
		return nil, fmt.Errorf("plan must not be nil")
	}

	totalItems := len(plan.Items)
	results := make([]BucketResult[T], totalItems)

	// Pre-fill hits from plan
	fetchIndices := make([]int, 0, len(plan.Fetches))
	for idx, item := range plan.Items {
		if item.State == StateValid && item.Entry != nil {
			results[idx] = BucketResult[T]{
				Interval:  item.Identity.Interval,
				Payload:   item.Entry.Payload,
				FromCache: true,
			}
		} else {
			fetchIndices = append(fetchIndices, idx)
		}
	}

	// If no fetches needed, return immediately
	if len(fetchIndices) == 0 {
		return &Result[T]{Buckets: results}, nil
	}

	if fetch == nil {
		return nil, fmt.Errorf("fetch callback must not be nil when cache misses exist")
	}

	// Bounded concurrency setup
	sem := make(chan struct{}, e.cfg.MaxConcurrency)
	var wg sync.WaitGroup

	errCh := make(chan error, len(fetchIndices))
	var cancelErr error
	var cancelMu sync.Mutex

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	fetchLoop:
	for _, idx := range fetchIndices {
		item := plan.Items[idx]

		// Check context before scheduling
		if ctx.Err() != nil {
			cancelMu.Lock()
			if cancelErr == nil {
				cancelErr = ctx.Err()
			}
			cancelMu.Unlock()
			break fetchLoop
		}

		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			cancelMu.Lock()
			if cancelErr == nil {
				cancelErr = ctx.Err()
			}
			cancelMu.Unlock()
			break fetchLoop
		}

		wg.Add(1)
		go func(resultIdx int, planItem PlanItem[T]) {
			defer func() {
				<-sem
				wg.Done()
			}()

			payload, err := fetch(ctx, planItem.Identity)
			if err != nil {
				bucketErr := BucketError{
					Interval: planItem.Identity.Interval,
					Err:      err,
				}
				select {
				case errCh <- bucketErr:
				default:
				}
				cancel()
				return
			}

			// Successful fetch: persist to cache store
			now := e.cfg.Clock()
			var expiresAt *time.Time
			if e.cfg.TTL > 0 {
				exp := now.Add(e.cfg.TTL)
				expiresAt = &exp
			}

			entry := CacheEntry[T]{
				Metadata: EntryMetadata{
					Identity:   planItem.Identity,
					Generation: e.cfg.Generation,
					CreatedAt:  now,
					ExpiresAt:  expiresAt,
					Completed:  true,
				},
				Payload: payload,
			}

			if e.store != nil {
				if writeErr := e.store.Set(ctx, entry); writeErr != nil {
					// Failure to write cache does not corrupt data, but should be reported if context canceled
					if ctx.Err() != nil {
						select {
						case errCh <- ctx.Err():
						default:
						}
						cancel()
						return
					}
				}
			}

			results[resultIdx] = BucketResult[T]{
				Interval:  planItem.Identity.Interval,
				Payload:   payload,
				FromCache: false,
			}
		}(idx, item)
	}

	wg.Wait()
	close(errCh)

	if cancelErr != nil {
		return nil, cancelErr
	}
	for err := range errCh {
		if err != nil {
			return nil, err
		}
	}

	return &Result[T]{Buckets: results}, nil
}
