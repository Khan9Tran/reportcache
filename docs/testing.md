# Testing strategy

Correctness is more important than maximizing cache hit rate.

## 1. Test layers

### Pure unit tests

Test interval and bucket logic without storage or network dependencies.

### Planner tests

Use fake cache entries and a deterministic validity policy. Verify the resulting plan.

### Executor tests

Use an in-memory fake cache and fetch callbacks that can return success, error, delay, or cancellation.

### Adapter contract tests

Run the same storage behavior tests against every adapter.

### Race tests

Exercise invalidation and refresh races, especially stale writers and generation changes.

## 2. Essential scenarios

### Time and bucket execution
- Entire requested range is cached and valid.
- Entire range is missing.
- Mixed valid and missing buckets.
- Mixed valid and invalidated buckets.
- Existing entry with unknown validity.
- Fetch failure for one bucket.
- Context cancelled during fetch.
- Cache write failure after successful fetch.
- Concurrent requests for the same bucket.
- Invalidation during an in-flight refresh.
- Old refresh finishes after a newer refresh.
- Partial first and last bucket.
- Timezone and daylight-saving boundaries.

### Scope identity and canonicalization
- **Same complete identity reuses cache:** Requests with identical `Namespace + ReportID + ScopeKey + Bucket + QueryVersion` hit the cache.
- **Scope isolation:** Requests with different office filters or scopes never collide or reuse each other's data.
- **Namespace isolation:** Identical report and filters under different namespaces (tenants) never collide.
- **Authorization isolation:** Different user authorization scopes produce distinct `ScopeKey` values and never leak data across permission boundaries.
- **Query version isolation:** Bumping `QueryVersion` partitions the cache; entries from older query versions are not reused.
- **Set-like filter order invariance:** `["office-002", "office-001"]` and `["office-001", "office-002"]` produce the exact same canonical `ScopeKey`.
- **Set-like deduplication invariance:** `["office-001", "office-002", "office-001"]` and `["office-001", "office-002"]` produce the exact same canonical `ScopeKey`.
- **Order-sensitive preservation:** Filters where order has business meaning (e.g. sort priorities, column projections) preserve their exact sequence in `ScopeKey`.
- **Absent vs populated distinction:** An omitted filter (unrestricted) and a populated filter (restricted) produce strictly distinct scope identities.
- **Empty filter validation:** An explicit empty list (`[]`) is explicitly rejected during validation and never silently treated as "all records".
- **Deterministic map serialization:** Map key serialization does not depend on randomized Go map iteration order.
- **Scope-format versioning:** Upgrading the application's canonicalization format version prevents incompatible cache entry reuse.

### Scope-aware invalidation
- **Dimension mutation invalidation:** Moving a record from Office A to Office B invalidates both Office A and Office B scopes (Strategy A) or advances a broader generation (Strategy B).
- **Combined scopes invalidation:** Multi-office scopes (e.g. Office A+B) are invalidated when a constituent record in Office A changes.
- **Broader generation advancement:** Advancing a report or namespace generation atomically invalidates all cached scopes under that report or namespace.
- **Unaffected scope preservation:** When using Strategy A, mutating Office A does not invalidate cached results for Office C.
- **Event-time plus dimension mutation:** Updating both event time and office filter invalidates old and new buckets across both old and new scopes.

## 3. Determinism

Avoid tests that depend on wall-clock timing or sleeps where possible.

Inject a clock only where the implementation needs current time for expiry or freshness decisions. Use channels/barriers to coordinate concurrency tests instead of relying on arbitrary sleep durations.

## 4. Property-style invariants

Where practical, test these invariants:
- resolved buckets are ordered;
- buckets do not overlap;
- the union of resolved buckets covers the requested range according to the selected edge policy;
- adjacent buckets share a boundary but do not overlap;
- requests with different `ScopeKey` values never share or overwrite cache entries;
- any permutation of a set-like filter produces an identical canonical `ScopeKey`;
- empty filter inputs are rejected before cache lookup and never treated as unrestricted;
- a dimension mutation invalidates both old and new scopes;
- a failed fetch is never stored as complete;
- an invalidated entry is never returned as a valid hit by the default policy.

## 5. Commands

```bash
gofmt -w .
go vet ./...
go test ./...
go test -race ./...
```
