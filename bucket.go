package reportcache

import (
	"fmt"
	"time"
)

// BucketSpec defines the strategy for dividing time intervals into deterministic buckets.
type BucketSpec interface {
	// ID returns a stable, unique identifier for this bucket strategy (e.g., "day:Asia/Ho_Chi_Minh", "fixed:15m").
	ID() string

	// Align returns the bucket start instant for the bucket containing instant t.
	Align(t time.Time) time.Time

	// Next returns the start instant of the bucket immediately following the bucket that starts at bucketStart.
	// bucketStart must already be aligned to a bucket boundary.
	Next(bucketStart time.Time) time.Time

	// Resolve decomposes interval into an ordered slice of contiguous buckets.
	// It returns ErrUnalignedInterval if interval.From or interval.To do not strictly align with bucket boundaries.
	Resolve(interval Interval) ([]Interval, error)
}

// FixedDurationBucket resolves intervals into fixed-duration buckets aligned with an anchor instant.
type FixedDurationBucket struct {
	duration time.Duration
	anchor   time.Time
}

// NewFixedDurationBucket creates a FixedDurationBucket.
// If anchor is zero, time.Unix(0, 0).UTC() is used as the default anchor.
func NewFixedDurationBucket(d time.Duration, anchor time.Time) (*FixedDurationBucket, error) {
	if d <= 0 {
		return nil, ErrInvalidBucketDuration
	}
	if anchor.IsZero() {
		anchor = time.Unix(0, 0).UTC()
	}
	return &FixedDurationBucket{
		duration: d,
		anchor:   anchor,
	}, nil
}

// ID returns the stable identifier for fixed-duration buckets.
func (b *FixedDurationBucket) ID() string {
	return fmt.Sprintf("fixed:%s", b.duration)
}

// Align aligns instant t with the beginning of its fixed-duration bucket relative to anchor.
func (b *FixedDurationBucket) Align(t time.Time) time.Time {
	diff := t.Sub(b.anchor)
	rem := diff % b.duration
	if rem < 0 {
		rem += b.duration
	}
	return t.Add(-rem)
}

// Next returns the start instant of the next bucket.
func (b *FixedDurationBucket) Next(bucketStart time.Time) time.Time {
	return bucketStart.Add(b.duration)
}

// Resolve decomposes interval into contiguous fixed-duration buckets.
func (b *FixedDurationBucket) Resolve(interval Interval) ([]Interval, error) {
	alignedFrom := b.Align(interval.From)
	if !alignedFrom.Equal(interval.From) {
		return nil, fmt.Errorf("%w: from %s is not aligned with bucket duration %s", ErrUnalignedInterval, interval.From, b.duration)
	}

	var buckets []Interval
	cur := alignedFrom
	for cur.Before(interval.To) {
		next := b.Next(cur)
		buckets = append(buckets, Interval{From: cur, To: next})
		cur = next
	}

	if !cur.Equal(interval.To) {
		return nil, fmt.Errorf("%w: to %s is not aligned with bucket duration %s", ErrUnalignedInterval, interval.To, b.duration)
	}

	return buckets, nil
}

// CalendarDayBucket resolves intervals into local calendar days in the specified IANA timezone.
type CalendarDayBucket struct {
	loc *time.Location
}

// NewCalendarDayBucket creates a CalendarDayBucket for the given timezone location.
func NewCalendarDayBucket(loc *time.Location) (*CalendarDayBucket, error) {
	if loc == nil {
		return nil, ErrNilLocation
	}
	return &CalendarDayBucket{loc: loc}, nil
}

// ID returns the stable identifier for calendar day buckets.
func (b *CalendarDayBucket) ID() string {
	return fmt.Sprintf("day:%s", b.loc.String())
}

// Align aligns instant t with local midnight (00:00:00) of that day in the configured location.
func (b *CalendarDayBucket) Align(t time.Time) time.Time {
	tInLoc := t.In(b.loc)
	return time.Date(tInLoc.Year(), tInLoc.Month(), tInLoc.Day(), 0, 0, 0, 0, b.loc)
}

// Next returns the local midnight of the subsequent calendar day.
// This handles 23-hour and 25-hour days across daylight saving transitions safely without adding 24 fixed hours.
func (b *CalendarDayBucket) Next(bucketStart time.Time) time.Time {
	startLoc := bucketStart.In(b.loc)
	return time.Date(startLoc.Year(), startLoc.Month(), startLoc.Day()+1, 0, 0, 0, 0, b.loc)
}

// Resolve decomposes interval into contiguous calendar days.
func (b *CalendarDayBucket) Resolve(interval Interval) ([]Interval, error) {
	alignedFrom := b.Align(interval.From)
	if !alignedFrom.Equal(interval.From) {
		return nil, fmt.Errorf("%w: from %s is not local midnight in timezone %s", ErrUnalignedInterval, interval.From, b.loc)
	}

	var buckets []Interval
	cur := alignedFrom
	for cur.Before(interval.To) {
		next := b.Next(cur)
		buckets = append(buckets, Interval{From: cur, To: next})
		cur = next
	}

	if !cur.Equal(interval.To) {
		return nil, fmt.Errorf("%w: to %s is not local midnight in timezone %s", ErrUnalignedInterval, interval.To, b.loc)
	}

	return buckets, nil
}
