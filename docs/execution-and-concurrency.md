# Execution and concurrency

## 1. Execution stages

A typical request follows these stages:

1. Validate request identity (`Namespace`, `ReportID`, `ScopeKey`, `QueryVersion`) and bucket configuration `[from, to)`.
2. Resolve requested range into ordered buckets.
3. Read cache entries and validity metadata for the specified scope.
4. Build a deterministic plan (hits, misses, stale, invalid).
5. Reuse valid entries.
6. Fetch missing or invalid buckets with bounded concurrency.
7. Store successful complete results with scope and validity metadata.
8. Return ordered bucket results and execution metadata.

Planning should be deterministic and testable independently from execution.

## 2. Bounded concurrency

Fetching multiple buckets can improve latency, but unbounded goroutines can overload the database or upstream service.

Provide a configurable maximum concurrency. The default should be conservative and documented.

The library must not create one unbounded goroutine per bucket.

## 3. Context cancellation

Pass `context.Context` to:
- cache reads and writes;
- validity checks that may perform I/O;
- fetch callbacks;
- lock or coordination operations.

When the context is cancelled, stop scheduling new work and cancel in-flight work where possible.

## 4. Failure semantics

If one bucket fetch fails:
- do not store it as complete;
- retain successful bucket writes only if the API clearly reports partial execution;
- return an error that identifies the failed bucket;
- do not silently claim the entire requested range is complete.

The first release should prefer all-or-error result semantics unless there is a clear requirement for partial results.

## 5. Stampede protection

Several requests may miss the same bucket and scope simultaneously.

Any flight coordination (such as process-local `singleflight` or optional distributed locks) must partition coordination keys by the complete five-dimension identity: `Namespace + ReportID + ScopeKey + Bucket + QueryVersion`. Requests for different filter combinations or tenants must never coalesce into the same flight.

Possible strategies:
- process-local singleflight;
- distributed lock;
- conditional write / generation check;
- tolerate duplicate fetches.

Do not require distributed locking in the core. It adds operational and failure complexity. Start with a simple safe behavior and make coordination optional.

A lock must have bounded waiting, lease expiry, cancellation behavior, and ownership-safe release. A lock expiry must not permit an old owner to overwrite a newer result.

## 6. Stale-while-revalidate

Serving stale data while refreshing can improve latency, but it weakens consistency.

Do not enable it by default. If added later, make it an explicit policy and return metadata that tells the caller the result is stale or being refreshed.

## 7. Observability

Consider lightweight hooks or result metadata for:
- bucket count;
- cache hit count;
- missing/invalid bucket count;
- fetch count and duration;
- cache read/write errors;
- refresh conflicts;
- partial execution.

Do not force a metrics library dependency. Expose hooks or structured metadata that applications can connect to their own observability stack.
