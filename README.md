# reportcache

**reportcache** is a time-range caching library for Go, designed for reports, analytics, dashboards, and time-based queries.

It helps an application reuse valid cached portions of a requested time range and fetch only the portions that are missing or need refreshing.

> Maintainer / author brand: **Khan9**

## The problem

A report API often receives a time range alongside business filters and authorization context:

```text
from     = 2026-11-01 00:00
to       = 2026-11-17 00:00
tenant   = company-123
offices  = ["office-001", "office-002"]
currency = VND
```

Querying the entire range and filter combination on every request can be expensive. Caching the entire response is also inefficient: a small change in one day or office may force the application to rebuild the whole report. At the same time, naive caching by date range alone is dangerous: two requests with different office filters or authorization scopes must never collide.

`reportcache` coordinates partial time-range reuse while isolating distinct filter scopes:

1. Identify the request by its complete context: `Namespace + ReportID + ScopeKey + Bucket + QueryVersion`.
2. Split the requested range into configured time buckets.
3. Determine which buckets have valid cached data for this specific scope.
4. Fetch missing or invalid buckets from the application's data source.
5. Store successfully fetched bucket results.
6. Return the bucket results to the application for domain-specific merging.

The library does **not** execute SQL, parse business filter maps, or decide how report rows should be aggregated. The application supplies canonical scope keys, fetch callbacks, cache storage, validity policies, and merge behavior through interfaces.

## Conceptual example

A report requests November 1–17 for `offices = ["office-001", "office-002"]`, using daily buckets.

- November 1–10: cached and valid for this filter scope.
- November 11–13: cache missing for this filter scope.
- November 14–16: cached but invalidated (e.g., a source record was modified).
- November 17: outside the requested half-open interval if `to` is midnight on November 17.

The planner reuses the valid buckets and asks the application to fetch only the missing and invalid buckets. Other scopes (such as `office-001` alone, or all offices) are cached independently.

## Design principles

- **Correctness first:** a cache hit is usable only when its validity can be established.
- **Time-range first:** partial range resolution is the core feature, not a generic key/value cache.
- **Scope and filter isolation:** cache entries are identified by the complete tuple `Namespace + ReportID + ScopeKey + Bucket + QueryVersion`. The application canonicalizes result-affecting filters into `ScopeKey`; the core treats it as an opaque identifier.
- **Independent scope caching (MVP):** each complete filter combination is cached independently. The core does not attempt automatic cross-scope subset composition (e.g., merging Office A and Office B to serve Office A+B).
- **Storage agnostic:** the core must not require Redis, PostgreSQL, MySQL, or a web framework.
- **Application-owned data semantics:** the caller decides how to merge, deduplicate, sort, or aggregate report payloads.
- **Explicit invalidation:** source data changes must be communicated to the cache layer or detected through a validity/version strategy.
- **Safe failure behavior:** failed or partial fetches must never be marked complete.
- **Small public API:** keep the initial API understandable and avoid speculative abstractions.

## Scope

### In scope

- Composite cache identity (`Namespace + ReportID + ScopeKey + Bucket + QueryVersion`).
- Fixed-duration and calendar-based time buckets.
- Half-open time intervals `[from, to)`.
- Planning valid, missing, stale, and invalidated bucket work.
- Fetching only buckets that need refresh.
- Storage interfaces and optional adapters.
- Invalidation and version-aware validity (exact scope invalidation and broader generation fallback).
- Context propagation, bounded concurrency, and cancellation.
- Tests for interval boundaries, scope isolation, canonicalization, and invalidation races.

### Not in the initial core

- SQL generation or ORM integration.
- Automatic report aggregation or generic row merging.
- Automatic cross-scope subset composition (e.g., combining cached Office A and Office B results to answer an Office A+B request).
- Dynamic business filter parsing or inspection in core.
- A mandatory Redis dependency.
- Distributed locking as a required feature.
- A scheduler or background worker.
- A universal solution for arbitrary non-time-based caching.

## Repository layout

```text
reportcache/
├── AGENTS.md
├── README.md
├── go.mod
├── .agent/
│   └── skills/
│       └── reportcache-development/
│           └── SKILL.md
├── docs/
│   ├── architecture.md
│   ├── api-design.md
│   ├── time-and-buckets.md
│   ├── cache-validity.md
│   ├── invalidation.md
│   ├── execution-and-concurrency.md
│   ├── storage-adapters.md
│   ├── testing.md
│   ├── decisions.md
│   └── roadmap.md
└── examples/
    └── report-flow.md
```

## Development status

This repository starts as a design-first project. Implement the pure time and planning core before adding a Redis adapter.

See [AGENTS.md](AGENTS.md), [architecture](docs/architecture.md), and [roadmap](docs/roadmap.md) before implementation.

## Naming

- Repository: `reportcache`
- Go package: `reportcache`
- Suggested module path: `github.com/Khan9Tran/reportcache`

The module path is provisional. Verify the actual Git hosting account name and repository URL before publishing or tagging a release.
