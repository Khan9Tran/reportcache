# Architecture

## 1. Goal

`reportcache` coordinates partial reuse of report data for a requested time range and filter scope. It is an orchestration library, not a report engine and not a database abstraction.

For a request identified by its complete context (`Namespace + ReportID + ScopeKey + Bucket + QueryVersion`) over `[from, to)`, it resolves the time range into buckets, determines which buckets can be reused, fetches the remaining buckets, and returns bucket-level results.

## 2. Responsibilities

### Core responsibilities

1. Validate and normalize the request identity tuple (`Namespace`, `ReportID`, `ScopeKey`, `Bucket`, `QueryVersion`) and time interval `[from, to)`.
2. Treat `ScopeKey` as an opaque, stable string without business filter inspection.
3. Resolve the request into deterministic buckets.
4. Read cache entries and their metadata for the requested scope.
5. Ask a validity policy whether each entry is reusable.
6. Build a plan containing cache hits and fetch work.
7. Execute fetch work with bounded concurrency.
8. Write only successfully completed results.
9. Return ordered bucket results and execution metadata.

### Application responsibilities

1. Own business filter semantics and canonicalize all result-affecting inputs (explicit filters + implicit constraints such as tenant, authorization, currency, timezone interpretation) into `ScopeKey`.
2. Define explicit semantics for absent versus empty filters.
3. Query the source of truth for requested bucket intervals and scopes.
4. Decide how to handle partial bucket edges.
5. Merge or aggregate bucket payloads.
6. Notify the cache about source mutations using scope-aware invalidation or advance a broader generation.
7. Choose acceptable freshness and consistency policy.

## 3. Suggested component model

```text
Application
    |
    | Request: (Namespace, ReportID, ScopeKey, QueryVersion) + [from, to) + bucket config
    v
Range Resolver
    |
    v
Cache Planner <------ Cache Metadata Store
    |
    | Plan: reusable buckets + fetch buckets (for ScopeKey)
    v
Executor ------------> Application Fetch Function
    |
    | successful bucket payloads
    v
Cache Writer
    |
    v
Ordered Bucket Results
    |
    v
Application Merge / Aggregate
```

The diagram is conceptual. The first implementation may combine planner and executor behind a small public API, but keep their responsibilities separate internally.

## 4. Main concepts

### Five-dimension cache identity

A cache entry is uniquely identified by the conceptual contract:

```text
Namespace + ReportID + ScopeKey + Bucket + QueryVersion
```

Each dimension has a dedicated responsibility:

- `Namespace`: tenant, company, or data isolation boundary.
- `ReportID`: logical report identity (e.g. `revenue`, `driver-activity`).
- `ScopeKey`: complete result-affecting filter and authorization scope.
- `Bucket`: time interval and its calendar or fixed-duration interpretation.
- `QueryVersion`: query semantics and cached payload compatibility.

This identity is a conceptual contract. The library does not unnecessarily expose internal cache key formatting as public API.

Do not key only by date range. Two requests with different office filters or authorization scopes must never collide.

### Scope ownership & opaque ScopeKey

The application owns business filter semantics. The `reportcache` core treats `ScopeKey` as an opaque, stable identifier.

The core must **not** understand or depend on business-specific filters such as:
- Office IDs.
- Department IDs.
- Payment methods.
- Reservation statuses.
- Partner IDs.
- Vehicle types.

The application is responsible for generating a canonical `ScopeKey` containing every input that can affect the report result. This includes both explicit filters and implicit constraints:
- Tenant/company restrictions.
- Authorization and data visibility.
- Default query conditions.
- Currency or locale when relevant.
- Timezone when it changes report interpretation.
- Other result-affecting business conditions.

Two requests may reuse the same cache entry only if their complete result-affecting scopes are equivalent.

### MVP caching strategy: independent scopes

For the MVP, cache each complete filter combination independently:

| Filter combination | Cached scope |
|---|---|
| All offices | Scope A |
| Office A | Scope B |
| Office B | Scope C |
| Office A + B | Scope D |

The core does **not** automatically compose Office A and Office B cached results to answer an Office A+B request.

**Reason:**
- The core cannot assume that arbitrary report payloads can be merged.
- Averages, distinct counts, rankings, percentiles, and pagination require domain-specific aggregation semantics.
- Business-specific merging belongs to the application.

Per-office caching and composition can be considered as a future optimization outside the initial core implementation.

### Time bucket

A bucket is a deterministic interval with a stable identity, such as a calendar day in `Asia/Ho_Chi_Minh`, or a fixed 15-minute interval in UTC.

### Cache entry

A cache entry contains:
- bucket identity and scope context;
- payload;
- creation/update metadata;
- validity metadata such as source version or generation;
- completion state.

Do not equate “entry exists” with “entry is valid”.

### Plan

A plan describes, for each bucket, whether it is reusable or must be fetched. It should be inspectable and deterministic.

### Fetch result

A fetch result represents a complete successful payload for one bucket. Partial success must be represented as an error or a distinct incomplete result, never as a completed cache entry.

## 5. Cache state model

The planner should distinguish at least:

| State | Meaning | Default action |
|---|---|---|
| Hit / valid | Entry exists and validity is proven | Reuse |
| Missing | No entry exists | Fetch |
| Invalidated | Explicitly marked dirty | Fetch |
| Stale | Freshness policy says refresh is needed | Fetch or serve-stale only if explicitly allowed |
| Unknown | Cannot prove validity | Fetch |
| In-progress | Another refresh is running | Wait, coordinate, or fetch according to policy |

Keep “stale-while-revalidate” out of the default behavior. It is a consistency policy and must be opt-in.

## 6. Package structure

Start small. Suggested initial structure:

```text
reportcache/
├── interval.go
├── bucket.go
├── planner.go
├── executor.go
├── cache.go
├── policy.go
├── errors.go
└── *_test.go
```

Do not create a package for every noun. Split packages only when there is a real dependency or API boundary.

Possible future adapters:

```text
adapters/
├── redis/
└── memory/
```

An in-memory adapter is useful for tests and examples. Redis should remain optional.

## 7. Design boundary: merging

The core must not assume that payloads can be concatenated.

Examples:
- Daily totals may be summed.
- An average requires sum and count, not the average of daily averages.
- Rows may need stable-ID deduplication and sorting.
- Pagination may require a global ordering operation.
- A report with joins or changing dimensions may need to be rebuilt as a whole.

Return ordered bucket-level results and let the caller merge them using domain knowledge.
