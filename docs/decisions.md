# Architecture decisions

This file records decisions that should remain consistent unless deliberately revised.

## D001 — Focus on report time ranges

**Decision:** The library focuses on caching report results split by time buckets.

**Reason:** This is the core user problem and differentiates the project from a generic key/value cache wrapper.

## D002 — Half-open intervals

**Decision:** All internal intervals use `[from, to)`.

**Reason:** Adjacent intervals are unambiguous and do not overlap.

## D003 — Core is storage agnostic

**Decision:** No mandatory Redis, SQL, ORM, or framework dependency in core.

**Reason:** Applications should be able to select their own storage backend.

## D004 — Application owns report merge semantics

**Decision:** Core returns ordered bucket results; the application merges or aggregates them.

**Reason:** Generic merge behavior is unsafe for averages, pagination, deduplication, and domain-specific calculations.

## D005 — Unknown validity is a miss by default

**Decision:** If validity cannot be proven, fetch the bucket.

**Reason:** Correctness is more important than cache hit ratio.

## D006 — TTL is not source consistency

**Decision:** TTL may be a freshness policy but cannot replace source versioning or invalidation where strict correctness is required.

**Reason:** Old records can be corrected or backfilled.

## D007 — Pure core before adapters

**Decision:** Implement interval resolution, planning, and tests before Redis or database adapters.

**Reason:** Core behavior should be validated without infrastructure complexity.

## D008 — No implicit stale-while-revalidate

**Decision:** Stale serving must be explicit and disabled by default.

**Reason:** It changes consistency guarantees.

## D009 — Safe refresh writes

**Decision:** Design generation/fencing semantics before claiming protection against concurrent stale writers.

**Reason:** A late refresh must not overwrite newer source state.

## D010 — Small first release

**Decision:** Defer scheduler, distributed locks, SQL helpers, ORM plugins, and automatic aggregation until a concrete need is demonstrated.

**Reason:** Avoid premature complexity and keep the library easy to adopt.

## D011 — Five-dimension cache identity

**Decision:** The conceptual cache identity consists of `Namespace + ReportID + ScopeKey + Bucket + QueryVersion`.

**Reason:** Isolates tenant data (`Namespace`), logical report identity (`ReportID`), complete filter/auth context (`ScopeKey`), temporal intervals (`Bucket`), and schema/semantics versioning (`QueryVersion`) unambiguously without leaking internal key formatting into public APIs.

## D012 — Opaque ScopeKey with application canonicalization

**Decision:** The core treats `ScopeKey` as an opaque, stable identifier. The application is strictly responsible for canonicalizing all result-affecting filters and constraints (handling absent vs empty filters, sorting sets, deterministic serialization).

**Reason:** Prevents coupling the core to domain-specific business filters (such as office, department, vehicle type, or status) while guaranteeing cache isolation.

## D013 — Independent scope caching for MVP

**Decision:** Cache each complete filter combination independently. The core will not automatically compose cached filter subsets (e.g. Office A + Office B) to answer broader requests (Office A+B).

**Reason:** Arbitrary report payloads cannot be generically merged for complex metrics (e.g., distinct counts, averages, percentiles, pagination). Sub-scope composition is deferred as a future application-level optimization.

## D014 — Scope-aware invalidation with generation fallback

**Decision:** When records mutate across filter dimensions or event times, applications must invalidate affected scopes (Strategy A) or advance a broader report/namespace generation (Strategy B). For the MVP, prefer correctness over cache hit ratio by falling back to broader generation advancement if exact fan-out cannot be guaranteed.

**Reason:** The core cannot infer subset/superset relationships from opaque `ScopeKey` strings. Broader generation advancement provides a safe, race-free guarantee of source consistency.
