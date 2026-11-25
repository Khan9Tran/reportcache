# Invalidation

Invalidation tells `reportcache` that previously cached report data may no longer represent the source of truth.

## 1. Why invalidation is necessary

Report data can change after the original event time:
- a reservation is cancelled;
- a payment is corrected;
- a refund is issued;
- an invoice is replaced;
- a record is backfilled;
- a source row is edited or deleted;
- a record moves between offices, departments, or categories.

A cache policy based only on age can miss these changes.

## 2. Which buckets and scopes should be invalidated?

A source mutation can change both temporal coordinates (event time) and dimensional coordinates (filters/attributes):

1. **Event-time change:**
   If a record's event time changes, invalidate the bucket containing the old event time and the bucket containing the new event time.
   - Example: A trip moved from November 10 to November 12 invalidates both November 10 and November 12 buckets.
2. **Dimension change:**
   If a record's business attributes change, invalidate scopes containing the old attributes and scopes containing the new attributes.
   - Example: A reservation moves from Office A to Office B.
3. **Deletions & Inserts:**
   For a delete, invalidate the old bucket and scope. For an insert, invalidate the new bucket and scope.

## 3. Scope-aware invalidation and fan-out

A single source mutation may affect multiple cached scopes simultaneously.

### The fan-out problem

Consider a reservation that moves from Office A to Office B. Potentially affected cached scopes include:
- All offices (unfiltered scope);
- Office A;
- Office B;
- Office A + B;
- Office A + C;
- Any custom filter combination containing either Office A or Office B.

Because `reportcache` core treats `ScopeKey` as an opaque, canonical string, **the library cannot infer these subset/superset relationships automatically**.

### Invalidation strategies

The application must select an invalidation strategy:

#### Strategy A: Exact Scope Invalidation

The application tracks or computes the exact set of active or affected canonical `ScopeKey` values and invalidates them explicitly.
- **Pros:** Preserves cache hits for unaffected scopes (e.g. Office C remains cached).
- **Cons:** Requires the application to maintain reverse indexes of active query scopes or compute full combinatorial fan-out.

#### Strategy B: Broader Generation Invalidation

The application advances a higher-level generation counter (at the `ReportID` or `Namespace` level). Because cache entries incorporate the generation into their validity checks, advancing this counter invalidates all cached scopes under that report or namespace in a single atomic step.
- **Pros:** Simple, robust, race-free, and guarantees source consistency without tracking query combinations.
- **Cons:** Flushes unaffected scopes, temporarily reducing cache hit ratio.

### MVP recommendation

- **Prefer correctness over hit ratio.**
- If exact scope fan-out cannot be tracked reliably by the application, **use broader generation invalidation (Strategy B)**.
- Do not claim automatic scope fan-out in the core unless an explicit index mechanism is implemented and verified.
- When mutating dimensions, always account for both old and new dimensions.
- When mutating event times, always account for both old and new time buckets.

## 4. Invalidation API concept

The library or adapter may expose operations such as:

```go
// Invalidate a specific scope and interval
InvalidateScope(ctx, namespace, reportID, scopeKey, interval)

// Advance generation for an entire report or namespace (Strategy B)
AdvanceReportGeneration(ctx, namespace, reportID)
AdvanceNamespaceGeneration(ctx, namespace)
```

These are conceptual names only. Final API design must account for the selected storage contract and generation model.

## 5. Invalidation implementation strategies

### Dirty marker

Store a dirty marker for a bucket. The planner treats the bucket as invalid until a successful refresh clears the marker.

Risk: clearing a dirty marker can race with a newer source mutation. Use generation or compare-and-swap semantics rather than blindly deleting a marker.

### Generation counter

Maintain a generation for a report scope or bucket. A cache entry records the generation used to build it. If current generation differs, the entry is invalid.

This is safer for concurrent refreshes when updates are atomic.

### Event-driven invalidation

The application publishes a domain event after a source mutation. A consumer maps the event to affected report keys and intervals.

The event consumer should be idempotent.

## 6. Dual-write consistency

A database transaction and a Redis invalidation write are usually not one atomic transaction.

Do not assume this sequence is reliable:

1. Commit source database transaction.
2. Write invalidation to Redis.

The process can crash between the two steps.

For applications that require reliable invalidation, recommend a transactional outbox:
1. Write the source mutation and outbox event in one database transaction.
2. A worker publishes/processes the event.
3. The consumer invalidates the affected report scope.
4. Retry safely until acknowledged.

`reportcache` should document this integration pattern; it should not implement an application's outbox.

## 7. Refresh race protection

Consider:
1. Refresh A starts using generation 5.
2. Source data changes; generation becomes 6.
3. Refresh B starts and writes generation 6.
4. Refresh A finishes late.

Refresh A must not overwrite generation 6 with generation 5.

Use a generation/fencing token and conditional write (CAS) where the storage supports it. If the adapter cannot guarantee this, document the limitation and choose a safe fallback.

## 8. Required tests

- Insert invalidates the new event-time bucket.
- Delete invalidates the old bucket.
- Event-time update invalidates both old and new buckets.
- Record moves between filter dimensions (e.g. Office A -> Office B), invalidating both Office A and Office B scopes.
- Combined scopes (e.g. Office A+B) are invalidated when a constituent record changes.
- Broader generation advancement (Strategy B) invalidates all scopes under that report/namespace.
- Unaffected scopes (e.g. Office C) remain valid under Strategy A.
- Duplicate invalidation event is safe (idempotent).
- Invalidation concurrent with refresh.
- Older refresh cannot overwrite newer generation.
- Retry after transient invalidation failure.
