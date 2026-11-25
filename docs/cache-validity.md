# Cache validity and freshness

## 1. Core rule

An existing cache entry is reusable only when the configured policy can establish that it represents current source data for the exact requested context to the required consistency level.

**Existence is not validity. TTL is not source consistency.**

Validity is strictly bound to the conceptual five-dimension cache identity:
```text
Namespace + ReportID + ScopeKey + Bucket + QueryVersion
```

A report for last month can still change because of late-arriving records, corrections, cancellations, refunds, or backfilled data. Similarly, cached data valid for one office or filter combination cannot be reused for a different filter combination or tenant.

## 2. Validity dimensions

A cache entry may carry:

- `Namespace`: isolation boundary.
- `ReportID`: logical report identity.
- `ScopeKey`: canonical filter and authorization scope.
- `Bucket`: temporal interval and timezone identity.
- `QueryVersion`: payload schema and query semantics version.
- `CreatedAt`: when this cache payload was written.
- `ExpiresAt`: optional time-based expiry.
- `Generation`: cache generation or fencing token (scope-level or broader).
- `SourceVersion`: version of the source data used to build the payload.
- `Watermark`: source completeness marker, if the application has one.
- `Completed`: whether the fetch completed successfully.
- `Invalidated`: explicit dirty marker or equivalent generation mismatch.

Do not require every application to provide every field. Define the minimum contract and let policies use additional metadata.

## 3. Validity strategies

### TTL-based freshness

Useful when the application accepts a bounded staleness window.

Example: refresh current-day report buckets every 30 seconds.

TTL says how old a cache entry may be; it does not prove that the source did not change.

### Explicit invalidation

The application marks affected bucket keys dirty after source mutations. The planner refreshes them on the next request or through a separately designed background process.

This is often the clearest model when the application knows which report periods and scopes were affected.

### Source version / generation

The application supplies a version representing the source state relevant to a bucket, scope, or report:

- **Scope-level generation:** Tracks changes specific to a single canonical `ScopeKey`. Advancing it invalidates cached buckets for that exact filter combination.
- **Broader generation (Report or Namespace):** Tracks changes across multiple scopes. Advancing it invalidates all cached entries across all scopes under that report or namespace (see [Invalidation](invalidation.md)).

A cache entry is valid only if its stored generation matches the current generation. The generation must advance whenever a relevant source mutation occurs.

### Watermark

A watermark can express that source data up to a point is considered complete. It is useful for event streams and late-arriving data, but it is only as trustworthy as the upstream system's watermark contract.

### Combined policy

A real report may combine explicit invalidation, a source version, and a short TTL for recent buckets.

Keep the policy explicit. Avoid a large set of implicit heuristics.

## 4. Unknown validity

If the cache entry exists but the library cannot establish its validity, treat it as unknown and fetch by default.

Serving unknown data must require an explicit opt-in policy with clearly documented consistency trade-offs.

## 5. Current and historical data

Do not hardcode a universal rule such as:
- current day is always stale;
- previous days are always valid;
- data older than seven days never changes.

These may be useful application policies, but they are not universal facts. The library should expose policy hooks rather than bake in domain assumptions.

## 6. Query and schema versioning

When the report query semantics or payload shape changes, old cache entries may no longer be compatible.

Use a caller-provided `QueryVersion` as part of the conceptual cache identity. Incrementing `QueryVersion` partitions the cache namespace, ensuring old entries are unreachable and cannot be deserialized into incompatible structs.

## 7. Policy test cases

- Matching source version and identical scope identity.
- Changed source version.
- Mismatching `ScopeKey` (different filter context is a miss).
- Scope-level generation advancement.
- Broader report/namespace generation advancement invalidating multiple scopes.
- Expired entry.
- Explicitly invalidated entry.
- Entry with unknown metadata.
- Incomplete entry.
- Query/schema version change.
- Historical bucket changed by a late correction.
