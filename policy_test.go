package reportcache_test

import (
	"testing"
	"time"

	"github.com/Khan9Tran/reportcache"
)

func TestGenerationPolicy(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	policy := reportcache.NewGenerationPolicy()

	t.Run("nil entry is missing", func(t *testing.T) {
		t.Parallel()
		state := policy.Evaluate(nil, reportcache.ValidityContext{CurrentTime: now})
		if state != reportcache.StateMissing {
			t.Errorf("got %v, want StateMissing", state)
		}
	})

	t.Run("incomplete entry is invalidated", func(t *testing.T) {
		t.Parallel()
		meta := &reportcache.EntryMetadata{
			Generation: 1,
			Completed:  false, // incomplete fetch!
		}
		state := policy.Evaluate(meta, reportcache.ValidityContext{CurrentTime: now})
		if state != reportcache.StateInvalidated {
			t.Errorf("got %v, want StateInvalidated", state)
		}
	})

	t.Run("older generation under Strategy B is invalidated", func(t *testing.T) {
		t.Parallel()
		meta := &reportcache.EntryMetadata{
			Generation: 1,
			Completed:  true,
		}
		state := policy.Evaluate(meta, reportcache.ValidityContext{
			CurrentTime:      now,
			ReportGeneration: 2, // generation bumped!
		})
		if state != reportcache.StateInvalidated {
			t.Errorf("got %v, want StateInvalidated", state)
		}
	})

	t.Run("older generation under Strategy A is invalidated", func(t *testing.T) {
		t.Parallel()
		meta := &reportcache.EntryMetadata{
			Generation: 1,
			Completed:  true,
		}
		state := policy.Evaluate(meta, reportcache.ValidityContext{
			CurrentTime:     now,
			ScopeGeneration: 2, // scope generation bumped!
		})
		if state != reportcache.StateInvalidated {
			t.Errorf("got %v, want StateInvalidated", state)
		}
	})

	t.Run("expired TTL is stale", func(t *testing.T) {
		t.Parallel()
		past := now.Add(-time.Minute)
		meta := &reportcache.EntryMetadata{
			Generation: 1,
			Completed:  true,
			ExpiresAt:  &past,
		}
		state := policy.Evaluate(meta, reportcache.ValidityContext{
			CurrentTime: now,
		})
		if state != reportcache.StateStale {
			t.Errorf("got %v, want StateStale", state)
		}
	})

	t.Run("matching generation and unexpired TTL is valid", func(t *testing.T) {
		t.Parallel()
		future := now.Add(time.Hour)
		meta := &reportcache.EntryMetadata{
			Generation: 2,
			Completed:  true,
			ExpiresAt:  &future,
		}
		state := policy.Evaluate(meta, reportcache.ValidityContext{
			CurrentTime:      now,
			ReportGeneration: 2,
			ScopeGeneration:  2,
		})
		if state != reportcache.StateValid {
			t.Errorf("got %v, want StateValid", state)
		}
	})
}
