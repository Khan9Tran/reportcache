# API design

This document defines the intended shape of the public API. It is a design target, not permission to implement every proposed type immediately.

## 1. Desired developer experience

A caller should be able to describe:
- what report is being cached (`ReportID`, `Namespace`);
- the result-affecting filter and authorization scope (`ScopeKey`);
- query semantics and schema compatibility (`QueryVersion`);
- the requested time range (`From`, `To`);
- the bucket strategy (`Bucket`);
- how to load one bucket;
- how validity is determined;
- how to store and retrieve bucket results.

The caller should not need to understand internal planner algorithms.

## 2. Conceptual usage

The exact API should be proposed and reviewed before implementation. A possible shape:

```go
type Request struct {
    ReportID     string
    Namespace    string
    ScopeKey     string
    QueryVersion string

    From   time.Time
    To     time.Time
    Bucket BucketSpec
}

type FetchFunc[T any] func(
    ctx context.Context,
    interval Interval,
) (T, error)

type Cache[T any] interface {
    Get(ctx context.Context, key string) (Entry[T], bool, error)
    Set(ctx context.Context, key string, entry Entry[T]) error
}
```

The public API may use Go generics if it makes typed payload storage straightforward. Do not introduce generic abstractions that make the API harder to understand.

## 3. Key design requirements

### Stable key construction

The library or adapter constructs a stable storage key from the conceptual five-dimension identity:
- `Namespace`: tenant/company/data boundary;
- `ReportID`: logical report identifier;
- `ScopeKey`: canonical filter and authorization context;
- `Bucket`: resolved interval start/end and calendar/timezone identity;
- `QueryVersion`: query semantics and payload compatibility.

The conceptual identity must remain private to the internal orchestration and adapter contracts rather than exposing raw key concatenation formats to applications.

### ScopeKey contract and canonicalization

The application owns business filter semantics. The core library treats `ScopeKey` as an opaque, stable string.

When an application derives `ScopeKey` from structured filters, it must follow these canonicalization rules:

1. **Normalize field names and values** according to the application's contract (e.g. trim whitespace, lowercase enum codes).
2. **Distinguish absent filters from explicit values.**
3. **Remove duplicates from set-like values.**
4. **Sort set-like values deterministically** (e.g. lexicographical order).
5. **Preserve ordering for order-sensitive values** (e.g. priority queues, ranking columns). Do not sort values whose order has business meaning.
6. **Serialize using deterministic encoding** (e.g. canonical JSON or sorted key-value pairs).
7. **Never rely on Go map iteration order**, which is randomized at runtime.
8. **Avoid ambiguous delimiter-based concatenation** (e.g. joining with `,` or `:` where values might contain delimiters; prefer escaping, length-prefixed encoding, or structured hashing).
9. **Hash long canonical representations when appropriate** (e.g. SHA-256) to ensure bounded key lengths in storage engines.
10. **Version the canonical format** if filter serialization semantics change over time.

Example for a set-like office filter:

```text
["office-002", "office-001"]
["office-001", "office-002", "office-001"]
```

Both input arrays must produce the exact same `ScopeKey`.

### Absent vs empty filter semantics

The application must define absent versus empty filter semantics explicitly.

Recommended contract for a set-like filter:

| Input | Meaning |
|---|---|
| Filter omitted / absent | No restriction (all records) |
| Explicit empty list (`[]`) | Invalid request (reject) |
| Non-empty list (`["A", "B"]`) | Restrict to specified values |

**Rules:**
- Never silently interpret an empty list as "all records".
- Absent and populated filters must generate different scope identities.
- If the application needs an explicit match-nothing condition, represent it explicitly rather than overloading empty lists.

### Fetch contract

Fetch receives the exact bucket interval and scope context. It must return a complete result for that interval or an error.

The core should not pass SQL fragments or database-specific objects to the callback.

### Cache contract

Storage should persist both payload and metadata needed to establish validity. Keep payload serialization outside the core unless a specific storage adapter requires it.

### Validity contract

A validity policy should be explicit and testable. It may compare:
- stored generation with current generation for the scope;
- stored source version with current source version;
- stored watermark with current watermark;
- freshness timestamps;
- explicit invalidation markers.

If no policy can prove validity, the safe default is to fetch.

## 4. Error behavior

Errors should preserve which bucket failed. A multi-bucket execution should make it possible to identify successful and failed buckets.

Do not return a normal complete result if one required bucket failed, unless the caller explicitly opts into partial-result semantics.

## 5. Unresolved API design decisions

Before finalizing exported APIs, the following design decisions require evaluation and explicit approval:

1. **Caller-provided `ScopeKey` string vs typed scope descriptor:**
   - *Option A:* Pass `ScopeKey string` directly in `Request`. Pros: core remains completely storage-agnostic and free of business filter types. Cons: caller must remember to canonicalize correctly.
   - *Option B:* Provide a typed builder or descriptor. Pros: enforces canonicalization discipline. Cons: risks expanding core API surface with reflection or dynamic map abstractions.
2. **Canonicalization helpers in core vs standalone package:**
   - Whether to provide optional canonicalization utilities (e.g., `reportcache/canonical` or `scopeutil`) to sort slices, encode sets, and hash long strings, keeping core orchestration free from encoding dependencies.
3. **Avoiding dynamic filter maps in core:**
   - The core must avoid accepting `map[string]any` directly. Should the library provide a strict key-value slice type for canonicalization, or leave that entirely to application code?
4. **Mandatory vs defaulted fields in `Request`:**
   - Should `Namespace` be strictly mandatory, or default to a safe constant (e.g. `"default"`)?
   - Should `QueryVersion` be mandatory to enforce schema discipline, or default to `"v1"`?

## 6. API review checklist

Before finalizing exported APIs, answer:
- Can a new Go developer understand the happy path quickly?
- Is `ScopeKey` clearly caller-owned and treated opaquely by the core?
- Are set sorting, deduplication, and absent vs empty filter rules enforced?
- Are timezone and bucket boundaries explicit?
- Can a caller use an in-memory cache without Redis?
- Can the caller test the fetch callback independently?
- Are partial failures visible?
- Can stale data only be served through an explicit policy?
- Are we exposing implementation details that should remain private?

Prefer fewer exported types and functions.
