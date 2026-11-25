package reportcache

import (
	"context"
	"fmt"
)

// Request defines a report query over a specified interval with full scope and identity dimensions.
type Request struct {
	Namespace    string
	ReportID     string
	ScopeKey     string
	QueryVersion string
	Interval     Interval
	Bucket       BucketSpec
}

// Validate checks that all request fields are populated and valid.
func (r Request) Validate() error {
	if r.Bucket == nil {
		return ErrNilBucketSpec
	}
	id := Identity{
		Namespace:    r.Namespace,
		ReportID:     r.ReportID,
		ScopeKey:     r.ScopeKey,
		BucketID:     r.Bucket.ID(),
		Interval:     r.Interval,
		QueryVersion: r.QueryVersion,
	}
	return id.Validate()
}

// PlanItem represents the planned action for a single bucket in the requested interval.
type PlanItem[T any] struct {
	Identity Identity
	State    ValidityState
	Entry    *CacheEntry[T]
}

// Plan contains the resolved buckets partitioned into reusable hits and required fetches.
type Plan[T any] struct {
	Request Request
	Items   []PlanItem[T]
	Hits    []PlanItem[T]
	Fetches []PlanItem[T]
}

// MetadataProvider retrieves cached entry for a given identity.
type MetadataProvider[T any] func(ctx context.Context, id Identity) (*CacheEntry[T], error)

// Planner decomposes requests into bucket-level plans using a validity policy.
type Planner[T any] struct {
	policy ValidityPolicy
}

// NewPlanner creates a new Planner with the specified validity policy.
// If policy is nil, NewGenerationPolicy() is used.
func NewPlanner[T any](policy ValidityPolicy) *Planner[T] {
	if policy == nil {
		policy = NewGenerationPolicy()
	}
	return &Planner[T]{policy: policy}
}

// Plan builds a deterministic plan for the given request.
func (p *Planner[T]) Plan(
	ctx context.Context,
	req Request,
	provider MetadataProvider[T],
	vCtx ValidityContext,
) (*Plan[T], error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	bucketIntervals, err := req.Bucket.Resolve(req.Interval)
	if err != nil {
		return nil, fmt.Errorf("bucket resolution failed: %w", err)
	}

	plan := &Plan[T]{
		Request: req,
		Items:   make([]PlanItem[T], 0, len(bucketIntervals)),
		Hits:    make([]PlanItem[T], 0),
		Fetches: make([]PlanItem[T], 0),
	}

	for _, bInterval := range bucketIntervals {
		id := Identity{
			Namespace:    req.Namespace,
			ReportID:     req.ReportID,
			ScopeKey:     req.ScopeKey,
			BucketID:     req.Bucket.ID(),
			Interval:     bInterval,
			QueryVersion: req.QueryVersion,
		}

		var meta *EntryMetadata
		var entry *CacheEntry[T]
		if provider != nil {
			var err error
			entry, err = provider(ctx, id)
			if err != nil {
				// On metadata read failure, treat as Unknown (safe miss fallback).
				meta = nil
			} else if entry != nil {
				meta = &entry.Metadata
			}
		}

		state := p.policy.Evaluate(meta, vCtx)
		item := PlanItem[T]{
			Identity: id,
			State:    state,
		}

		if state == StateValid && entry != nil {
			item.Entry = entry
			plan.Hits = append(plan.Hits, item)
		} else {
			plan.Fetches = append(plan.Fetches, item)
		}

		plan.Items = append(plan.Items, item)
	}

	return plan, nil
}
