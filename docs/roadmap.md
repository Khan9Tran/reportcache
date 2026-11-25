# Roadmap

The roadmap is ordered by dependency and correctness, not by the number of features.

## Phase 0 — Design review

- [ ] Review the proposed public API and `Request` struct (`Namespace`, `ReportID`, `ScopeKey`, `QueryVersion`, `BucketSpec`).
- [ ] Review `ScopeKey` contract, canonicalization rules, and absent vs empty filter semantics.
- [ ] Evaluate unresolved API decisions (caller string vs typed descriptor, canonicalization helper package, mandatory vs defaulted fields).
- [ ] Decide fixed-duration versus calendar bucket MVP.
- [ ] Decide exact partial-edge behavior.
- [ ] Define minimum cache entry metadata.
- [ ] Define validity policy interface.
- [ ] Define generation and conditional-write contract.
- [ ] Confirm module path and repository ownership.

**Exit criteria:** documented decisions and an approved implementation plan.

## Phase 1 — Pure time and identity core

- [ ] Interval type and validation.
- [ ] Five-dimension cache identity model (`Namespace + ReportID + ScopeKey + Bucket + QueryVersion`).
- [ ] Fixed-duration bucket resolver.
- [ ] Calendar-day resolver with explicit timezone.
- [ ] Stable bucket identity.
- [ ] Boundary and DST tests.

**Exit criteria:** deterministic bucket resolution and identity construction with no storage dependency.

## Phase 2 — Planner

- [ ] Cache entry metadata model with scope and generation awareness.
- [ ] Validity policy contract (supporting scope-level and broader generation checks).
- [ ] Hit/missing/invalidated/stale/unknown states.
- [ ] Deterministic plan output.
- [ ] Scope isolation and canonicalization invariant tests.
- [ ] Planner tests.

**Exit criteria:** given a request and cache metadata, planner always produces a correct, inspectable plan.

**Exit criteria:** given a request and cache metadata, planner always produces a correct, inspectable plan.

## Phase 3 — Executor

- [ ] Fetch callback.
- [ ] Bounded concurrency.
- [ ] Context cancellation.
- [ ] Safe successful writes only.
- [ ] Bucket-specific errors.
- [ ] Partial-failure semantics.

**Exit criteria:** mixed cache/fetch requests work with fake storage and complete tests.

## Phase 4 — In-memory adapter and examples

- [ ] In-memory cache for tests/examples.
- [ ] Example report flow.
- [ ] Public API documentation.
- [ ] Benchmarks for planner overhead.

**Exit criteria:** a developer can understand and run a complete example without Redis.

## Phase 5 — Redis adapter (optional)

- [ ] Redis key schema.
- [ ] Serialization format.
- [ ] TTL and metadata storage.
- [ ] Generation/CAS behavior.
- [ ] Adapter contract tests.
- [ ] Redis outage and timeout behavior.

**Exit criteria:** adapter guarantees and limitations are documented and tested.

## Phase 6 — Production hardening

- [ ] Observability hooks.
- [ ] Optional singleflight or distributed coordination.
- [ ] Fuzz/property tests for interval resolution.
- [ ] Race tests for invalidation and refresh.
- [ ] Compatibility/versioning policy.
- [ ] Go package documentation and release workflow.

## Explicitly deferred

- Automatic SQL generation.
- ORM-specific integrations.
- Automatic report aggregation.
- Automatic cross-scope subset composition (e.g. composing Office A and Office B to serve Office A+B).
- Built-in scheduler.
- Mandatory distributed lock.
- Multi-tier cache unless a real use case justifies it.
