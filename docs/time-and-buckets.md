# Time ranges and buckets

Time handling is a correctness-critical part of `reportcache`.

## 1. Interval convention

Use half-open intervals everywhere:

```text
[from, to)
```

The start is included; the end is excluded.

Benefits:
- Adjacent intervals do not overlap.
- A timestamp exactly at `to` belongs to the next interval.
- Duration and boundary behavior are easier to reason about.

Reject invalid intervals where `from >= to`, unless a separately documented API explicitly supports empty ranges.

## 2. Normalize instants, preserve calendar meaning

A `time.Time` represents an instant and carries location information. The library must not silently assume the machine's local timezone.

The caller must choose the timezone used for calendar buckets. For example:
- UTC for globally aligned data;
- `Asia/Ho_Chi_Minh` for Vietnam-local daily reports.

Do not silently convert calendar boundaries using the host timezone.

## 3. Bucket types

Potential supported strategies:

### Fixed duration

Examples: 5 minutes, 1 hour, 24 hours.

A fixed-duration bucket is aligned to a defined anchor, usually Unix epoch or an explicitly configured anchor. Document the alignment rule.

### Calendar day

A day begins at local midnight in a configured IANA timezone and ends at the next local midnight.

Do not calculate the next calendar day by adding exactly 24 hours. Daylight-saving transitions can create 23-hour or 25-hour local days.

### Calendar week

The caller must configure:
- timezone;
- first day of week;
- week boundary convention.

### Calendar month

A month starts on the first local day of the month and ends at the first local day of the next month.

### Calendar year

A year starts at local January 1 and ends at the next local January 1.

### Custom calendar units

Do not add arbitrary calendar rules to the first release unless a real use case requires them.

## 4. Partial first and last buckets

A requested range may begin or end inside a bucket.

Example: request `[2026-11-03 10:00, 2026-11-05 15:00)` with daily buckets.

The first and last buckets are partial relative to the request. A cache entry for the entire day cannot automatically answer a partial-day request unless:
- the cached payload can be filtered accurately to the requested subrange; or
- the application fetches the edge interval separately; or
- the cache granularity is sufficiently fine.

The API must make this explicit. Never silently return a whole-day aggregate for a partial-day request.

## 5. Bucket identity within cache identity

Within the conceptual five-dimension cache identity:

```text
Namespace + ReportID + ScopeKey + Bucket + QueryVersion
```

The `Bucket` dimension represents the temporal component. A bucket identity must be deterministic and include enough information to avoid collisions:
- bucket strategy (e.g. calendar day, calendar month, 15m fixed duration);
- timezone or fixed-duration anchor;
- start instant and end instant `[from, to)`.

Do not rely on a formatted date string alone (such as `"2026-11-03"`), which is ambiguous without timezone and boundary context. The caller's report filters, tenant, and permissions are captured in `ScopeKey`, `Namespace`, and `ReportID`, leaving `Bucket` focused purely on time.

## 6. Required tests

- Exact bucket boundary.
- One nanosecond before and after a boundary.
- Adjacent intervals.
- Range shorter than one bucket.
- Partial first and last bucket.
- Month and year transitions.
- Leap day.
- DST transition in a DST-observing timezone.
- Different timezone configurations for the same instant.
- Invalid and empty intervals.
