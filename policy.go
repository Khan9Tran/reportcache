package reportcache

import (
	"fmt"
	"time"
)

// ValidityState describes whether a cache entry can be reused or must be fetched.
type ValidityState int

const (
	// StateValid indicates the entry exists and its validity is proven.
	StateValid ValidityState = iota
	// StateMissing indicates no cache entry exists for the requested identity.
	StateMissing
	// StateInvalidated indicates the entry exists but has been marked dirty or superseded by a newer generation.
	StateInvalidated
	// StateStale indicates the entry has passed its TTL freshness window.
	StateStale
	// StateUnknown indicates the entry exists but validity cannot be proven (treated as a miss by default).
	StateUnknown
)

// String returns the human-readable representation of ValidityState.
func (s ValidityState) String() string {
	switch s {
	case StateValid:
		return "Valid"
	case StateMissing:
		return "Missing"
	case StateInvalidated:
		return "Invalidated"
	case StateStale:
		return "Stale"
	case StateUnknown:
		return "Unknown"
	default:
		return fmt.Sprintf("ValidityState(%d)", s)
	}
}

// ValidityContext carries environment and generation state evaluated by a ValidityPolicy.
type ValidityContext struct {
	// CurrentTime is used to evaluate TTL expiration. If zero, TTL check is skipped.
	CurrentTime time.Time

	// ReportGeneration is the current generation counter at the report level (Strategy B).
	// If the entry's generation is lower than ReportGeneration, it is considered Invalidated.
	ReportGeneration int64

	// ScopeGeneration is the generation counter for the specific scope (Strategy A).
	// If the entry's generation is lower than ScopeGeneration, it is considered Invalidated.
	ScopeGeneration int64
}

// ValidityPolicy evaluates whether a cache entry is reusable or needs refreshing.
type ValidityPolicy interface {
	Evaluate(meta *EntryMetadata, ctx ValidityContext) ValidityState
}

// GenerationPolicy is the standard validity policy checking generation counters and TTL.
type GenerationPolicy struct{}

// NewGenerationPolicy creates a new GenerationPolicy.
func NewGenerationPolicy() *GenerationPolicy {
	return &GenerationPolicy{}
}

// Evaluate determines the validity of meta against the given context.
func (p *GenerationPolicy) Evaluate(meta *EntryMetadata, ctx ValidityContext) ValidityState {
	if meta == nil {
		return StateMissing
	}

	// Incomplete fetches must never be treated as valid hits.
	if !meta.Completed {
		return StateInvalidated
	}

	// Strategy B: Broader report generation check.
	if ctx.ReportGeneration > 0 && meta.Generation < ctx.ReportGeneration {
		return StateInvalidated
	}

	// Strategy A: Scope-specific generation check.
	if ctx.ScopeGeneration > 0 && meta.Generation < ctx.ScopeGeneration {
		return StateInvalidated
	}

	// TTL check.
	if meta.ExpiresAt != nil && !ctx.CurrentTime.IsZero() && (ctx.CurrentTime.Equal(*meta.ExpiresAt) || ctx.CurrentTime.After(*meta.ExpiresAt)) {
		return StateStale
	}

	return StateValid
}
