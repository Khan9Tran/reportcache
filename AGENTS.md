# Instructions for coding agents

You are working on **reportcache**, a Go library for partial time-range caching of reports.

## Mandatory workflow

1. Read this file and every document under `docs/`.
2. Inspect the existing repository before proposing changes.
3. Summarize the problem and the proposed implementation.
4. Identify correctness risks and unresolved design decisions.
5. Produce a step-by-step implementation plan with files, APIs, and tests.
6. **Wait for explicit approval before writing implementation code** unless the user explicitly asks you to implement.
7. After approval, implement in small, testable increments.

Do not skip analysis and do not start by generating a large framework.

## Product definition

Given a report request identified by its complete context (`Namespace + ReportID + ScopeKey + Bucket + QueryVersion`) over `[from, to)`, the library coordinates reuse of cached time buckets that are known to be valid and fetching of buckets that are missing, stale, or invalidated.

The application owns:
- Business filter semantics and canonicalization into an opaque `ScopeKey`.
- Explicit handling of absent versus empty filters.
- Fetching report data from its database or upstream service.
- The meaning and shape of report payloads.
- Merging, aggregating, sorting, and deduplicating payloads.
- Emitting scope-aware invalidation events or advancing generations when source data or dimensions change.

The library owns:
- Conceptual cache identity contract validation.
- Treating `ScopeKey` as an opaque, stable string without business filter inspection.
- Time-range normalization and bucket resolution.
- Cache planning.
- Cache execution orchestration.
- Cache metadata and validity decisions.
- Safe storage contracts and concurrency controls.

## Non-negotiable correctness rules

- Use half-open intervals `[from, to)` internally.
- Complete cache identity is `Namespace + ReportID + ScopeKey + Bucket + QueryVersion`.
- Never assume two requests with different filter or authorization contexts can share cache entries. Requests may share cache entries only when their complete result-affecting scopes are equivalent.
- Treat `ScopeKey` as an opaque, canonical identifier owned by the application; the core must never inspect, parse, or guess business filter semantics.
- Never silently interpret an explicit empty filter list as "all records" or "unfiltered"; absent filters (no restriction) and explicit empty filters (invalid request) must produce distinct semantics and identities.
- Cache each complete filter combination independently in the MVP; do not attempt automatic cross-scope subset composition (e.g., composing Office A and Office B to serve Office A+B) in core.
- When source records mutate filter dimensions or authorization attributes, invalidate all affected scopes or advance a broader generation covering both old and new dimensions.
- If event time changes, invalidate both old and new time buckets.
- Never assume old data is immutable merely because it is historical.
- TTL alone does not prove source-data consistency.
- Never mark a bucket complete after a failed, cancelled, or partial fetch.
- Do not serve an invalidated or unknown-validity bucket as valid by default.
- Do not let a slower, older refresh overwrite a newer generation.
- Handle partial first and last buckets explicitly. Do not use a whole-month aggregate to answer a partial-month request unless the payload can be filtered or the application fetches the edge separately.
- Propagate `context.Context` through all potentially blocking operations.
- Bound concurrency; do not launch one goroutine per bucket without a limit.
- Do not hide data correctness decisions inside a generic merge function.

## Architecture constraints

- Core packages must use only the Go standard library unless a dependency is justified and approved.
- Redis, SQL drivers, ORMs, HTTP frameworks, and cloud SDKs must not be imported by the core.
- Storage is defined by small interfaces.
- Optional adapters live in separate packages or modules.
- Avoid reflection-heavy APIs and `any`-based abstractions unless there is a concrete need.
- Prefer explicit types and simple interfaces.

## Coding standards

- Follow idiomatic Go and keep exported APIs small.
- Document exported identifiers.
- Return errors with useful context; wrap errors with `%w`.
- Avoid global mutable state.
- Avoid implicit timezone assumptions.
- Avoid silently coercing incompatible bucket configurations.
- Make deterministic behavior easy to test with an injected clock where time-dependent behavior requires it.
- Add table-driven tests for boundary conditions.
- Run `gofmt`, `go vet`, and `go test ./...` when implementation exists.

## Before adding a feature

Explain:
- Which user problem it solves.
- Whether it belongs in core or an adapter.
- How it affects correctness and API complexity.
- How it is tested.
- Whether a simpler implementation already solves the problem.

## Definition of done

A feature is not done until:
- Its behavior is documented.
- Its edge cases are tested.
- Failure behavior is explicit.
- It does not violate the architecture constraints.
- The full test suite passes.
