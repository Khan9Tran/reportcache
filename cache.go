package reportcache

import (
	"context"
	"time"
)

// BucketResult contains the payload for a single bucket and indicates whether it was served from cache.
type BucketResult[T any] struct {
	Interval  Interval
	Payload   T
	FromCache bool
}

// Result contains the ordered bucket results for a request.
type Result[T any] struct {
	Buckets []BucketResult[T]
}

// CacheStore is the storage abstraction for persisting and retrieving cache entries.
type CacheStore[T any] interface {
	Get(ctx context.Context, id Identity) (*CacheEntry[T], error)
	Set(ctx context.Context, entry CacheEntry[T]) error
}

// Option configures reportcache execution.
type Option func(*options)

type options struct {
	maxConcurrency   int
	generation       int64
	reportGeneration int64
	scopeGeneration  int64
	ttl              time.Duration
	clock            func() time.Time
	policy           ValidityPolicy
}

func defaultOptions() options {
	return options{
		maxConcurrency: 4,
		generation:     1,
		clock:          time.Now,
	}
}

// WithMaxConcurrency sets the maximum number of concurrent fetch operations.
func WithMaxConcurrency(n int) Option {
	return func(o *options) {
		if n > 0 {
			o.maxConcurrency = n
		}
	}
}

// WithGeneration sets the current generation number when storing fetched entries.
func WithGeneration(g int64) Option {
	return func(o *options) {
		if g > 0 {
			o.generation = g
		}
	}
}

// WithReportGeneration sets the active report generation counter for Strategy B invalidation check.
func WithReportGeneration(g int64) Option {
	return func(o *options) {
		o.reportGeneration = g
	}
}

// WithScopeGeneration sets the active scope generation counter for Strategy A invalidation check.
func WithScopeGeneration(g int64) Option {
	return func(o *options) {
		o.scopeGeneration = g
	}
}

// WithTTL sets the time-to-live for cached bucket entries.
func WithTTL(d time.Duration) Option {
	return func(o *options) {
		o.ttl = d
	}
}

// WithClock injects a deterministic time source, useful for testing expiration.
func WithClock(fn func() time.Time) Option {
	return func(o *options) {
		if fn != nil {
			o.clock = fn
		}
	}
}

// WithValidityPolicy sets a custom validity policy for the planner.
func WithValidityPolicy(p ValidityPolicy) Option {
	return func(o *options) {
		if p != nil {
			o.policy = p
		}
	}
}

// Resolve orchestrates range decomposition, cache planning, bounded fetch execution,
// and safe caching for a report request.
func Resolve[T any](
	ctx context.Context,
	req Request,
	store CacheStore[T],
	fetch FetchFunc[T],
	opts ...Option,
) (*Result[T], error) {
	cfg := defaultOptions()
	for _, opt := range opts {
		opt(&cfg)
	}

	planner := NewPlanner[T](cfg.policy)

	vCtx := ValidityContext{
		CurrentTime:      cfg.clock(),
		ReportGeneration: cfg.reportGeneration,
		ScopeGeneration:  cfg.scopeGeneration,
	}

	var metadataProvider MetadataProvider[T]
	if store != nil {
		metadataProvider = func(ctx context.Context, id Identity) (*CacheEntry[T], error) {
			return store.Get(ctx, id)
		}
	}

	plan, err := planner.Plan(ctx, req, metadataProvider, vCtx)
	if err != nil {
		return nil, err
	}

	execCfg := ExecutorConfig{
		MaxConcurrency: cfg.maxConcurrency,
		Generation:     cfg.generation,
		TTL:            cfg.ttl,
		Clock:          cfg.clock,
	}

	executor := NewExecutor[T](store, execCfg)
	return executor.Execute(ctx, plan, fetch)
}
