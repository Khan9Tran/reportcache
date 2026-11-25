# Example: report request flow

This example describes the intended integration. It is illustrative, not a finalized compilable API.

## Scenario

A revenue report API receives a request for a date range with multi-office filters, tenant context, and authorization constraints:

- Tenant: `company-123`
- Report: `daily-revenue`
- Offices requested: `["office-002", "office-001"]`
- Date range: `[2026-11-01 00:00, 2026-11-17 00:00)` in `Asia/Ho_Chi_Minh`
- Currency: `VND`
- Query version: `v2`

The application wants to reuse daily cached results where safe, query only days that need refreshing, and guarantee that distinct office filter combinations never collide.

## Application responsibilities: canonicalizing ScopeKey

The application validates filters and constructs a canonical `ScopeKey`:

1. **Absent vs empty filter validation:**
   - If `offices` query param is omitted: unrestricted (all offices).
   - If `offices` is an explicit empty array `[]`: reject as invalid request (`ErrInvalidFilter`).
   - If `offices` is populated: proceed with canonicalization.
2. **Set canonicalization:**
   - De-duplicate and sort lexicographically: `["office-002", "office-001"]` -> `["office-001", "office-002"]`.
3. **Include implicit constraints:**
   - Include currency (`currency=VND`) and user visibility scope.
4. **Deterministic serialization:**
   - Serialize deterministically (e.g. `offices=office-001,office-002|currency=VND`) or hash if long (e.g. SHA-256).

```go
// Application code canonicalizing the scope
func buildScopeKey(offices []string, currency string) (string, error) {
    if offices != nil && len(offices) == 0 {
        return "", errors.New("empty office list is invalid")
    }
    
    // De-duplicate and sort set-like filters
    set := make(map[string]struct{}, len(offices))
    for _, o := range offices {
        set[o] = struct{}{}
    }
    sorted := make([]string, 0, len(set))
    for o := range set {
        sorted = append(sorted, o)
    }
    sort.Strings(sorted) // deterministic order

    return fmt.Sprintf("offices=%s;currency=%s", strings.Join(sorted, ","), currency), nil
}
```

## Request flow

The application passes the canonical `ScopeKey` and identity dimensions in `reportcache.Request`:

```go
// Pseudocode: API names are illustrative.
scopeKey, err := buildScopeKey(req.Offices, "VND")
if err != nil {
    return nil, err
}

result, err := reportcache.Resolve(ctx, reportcache.Request{
    Namespace:    "company-123",
    ReportID:     "daily-revenue",
    ScopeKey:     scopeKey,
    QueryVersion: "v2",
    From:         from,
    To:           to,
    Bucket:       reportcache.CalendarDay("Asia/Ho_Chi_Minh"),
}, reportcache.WithFetch(func(ctx context.Context, bucket reportcache.Interval) (DailyRevenue, error) {
    // Query only this bucket from the database for the given scope
    return repository.GetDailyRevenue(ctx, bucket.From, bucket.To, req.Offices)
}))
if err != nil {
    return nil, err
}

// The application owns domain-specific merge semantics.
summary := mergeDailyRevenue(result.Buckets)
```

## Important edge case

If the request is from 10:00 on the first day to 15:00 on the last day, a whole-day aggregate is not automatically sufficient for either edge.

Possible solutions:
- fetch partial edge intervals separately;
- cache finer-grained data;
- store filterable raw records;
- reject unsupported partial-edge requests.

Choose and document one behavior. Never silently include data outside the requested interval.

## Mutation and invalidation example

Suppose a reservation moves from Office A to Office B, and its event date shifts from November 10 to November 12:

1. **Both event dates are affected:** November 10 and November 12 buckets must be refreshed.
2. **Both office scopes are affected:** Cached scopes for Office A, Office B, and combinations containing them are stale.

The application chooses its invalidation strategy:

- **Strategy A (Exact Scope Invalidation):** If the application tracks active query scopes, it invalidates buckets for Scope(Office A), Scope(Office B), and Scope(Office A+B).
- **Strategy B (Broader Generation Invalidation — Recommended for MVP):** If tracking every combination is impractical, the application increments the generation for `Namespace: "company-123", ReportID: "daily-revenue"`. All existing cached scopes for this report immediately become stale in a race-free manner.

For reliable delivery, source mutations and invalidation events should be recorded together in a transactional outbox.
