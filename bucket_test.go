package reportcache_test

import (
	"errors"
	"testing"
	"time"

	"github.com/Khan9/reportcache"
)

func TestFixedDurationBucket(t *testing.T) {
	t.Parallel()

	// 1-hour fixed duration anchored at 00:00 UTC
	anchor := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	b, err := reportcache.NewFixedDurationBucket(time.Hour, anchor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if b.ID() != "fixed:1h0m0s" {
		t.Errorf("ID() = %v, want fixed:1h0m0s", b.ID())
	}

	// Invalid duration
	if _, err := reportcache.NewFixedDurationBucket(0, anchor); !errors.Is(err, reportcache.ErrInvalidBucketDuration) {
		t.Errorf("want ErrInvalidBucketDuration for 0 duration, got %v", err)
	}

	// Align check
	tMid := time.Date(2026, 1, 1, 3, 25, 30, 100, time.UTC)
	aligned := b.Align(tMid)
	expectedAlign := time.Date(2026, 1, 1, 3, 0, 0, 0, time.UTC)
	if !aligned.Equal(expectedAlign) {
		t.Errorf("Align() = %v, want %v", aligned, expectedAlign)
	}

	// Resolve aligned 3-hour interval
	from := time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 1, 4, 0, 0, 0, time.UTC)
	iv, _ := reportcache.NewInterval(from, to)

	buckets, err := b.Resolve(iv)
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}
	if len(buckets) != 3 {
		t.Fatalf("expected 3 buckets, got %d", len(buckets))
	}
	if !buckets[0].From.Equal(from) || !buckets[2].To.Equal(to) {
		t.Errorf("buckets boundary mismatch: [%v, %v]", buckets[0].From, buckets[2].To)
	}

	// Resolve unaligned start
	unalignedFrom := time.Date(2026, 1, 1, 1, 15, 0, 0, time.UTC)
	ivUnalignedFrom, _ := reportcache.NewInterval(unalignedFrom, to)
	if _, err := b.Resolve(ivUnalignedFrom); !errors.Is(err, reportcache.ErrUnalignedInterval) {
		t.Errorf("want ErrUnalignedInterval for unaligned From, got %v", err)
	}

	// Resolve unaligned end
	unalignedTo := time.Date(2026, 1, 1, 4, 45, 0, 0, time.UTC)
	ivUnalignedTo, _ := reportcache.NewInterval(from, unalignedTo)
	if _, err := b.Resolve(ivUnalignedTo); !errors.Is(err, reportcache.ErrUnalignedInterval) {
		t.Errorf("want ErrUnalignedInterval for unaligned To, got %v", err)
	}
}

func TestCalendarDayBucket(t *testing.T) {
	t.Parallel()

	hcm, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Fatalf("failed to load location: %v", err)
	}

	b, err := reportcache.NewCalendarDayBucket(hcm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if b.ID() != "day:Asia/Ho_Chi_Minh" {
		t.Errorf("ID() = %v, want day:Asia/Ho_Chi_Minh", b.ID())
	}

	// Nil location
	if _, err := reportcache.NewCalendarDayBucket(nil); !errors.Is(err, reportcache.ErrNilLocation) {
		t.Errorf("want ErrNilLocation, got %v", err)
	}

	// Alignment
	midDay := time.Date(2026, 11, 15, 14, 30, 0, 0, hcm)
	aligned := b.Align(midDay)
	expectedAlign := time.Date(2026, 11, 15, 0, 0, 0, 0, hcm)
	if !aligned.Equal(expectedAlign) {
		t.Errorf("Align() = %v, want %v", aligned, expectedAlign)
	}

	// Resolve 5 days
	from := time.Date(2026, 11, 1, 0, 0, 0, 0, hcm)
	to := time.Date(2026, 11, 6, 0, 0, 0, 0, hcm)
	iv, _ := reportcache.NewInterval(from, to)

	days, err := b.Resolve(iv)
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}
	if len(days) != 5 {
		t.Fatalf("expected 5 daily buckets, got %d", len(days))
	}
	for i := 0; i < len(days); i++ {
		expectedDayStart := time.Date(2026, 11, 1+i, 0, 0, 0, 0, hcm)
		expectedDayEnd := time.Date(2026, 11, 2+i, 0, 0, 0, 0, hcm)
		if !days[i].From.Equal(expectedDayStart) || !days[i].To.Equal(expectedDayEnd) {
			t.Errorf("bucket %d mismatch: got [%v, %v), want [%v, %v)",
				i, days[i].From, days[i].To, expectedDayStart, expectedDayEnd)
		}
	}

	// Leap year test (February 2024 has 29 days)
	fromLeap := time.Date(2024, 2, 28, 0, 0, 0, 0, hcm)
	toLeap := time.Date(2024, 3, 2, 0, 0, 0, 0, hcm)
	ivLeap, _ := reportcache.NewInterval(fromLeap, toLeap)

	leapBuckets, err := b.Resolve(ivLeap)
	if err != nil {
		t.Fatalf("Resolve() leap year unexpected error: %v", err)
	}
	if len(leapBuckets) != 3 {
		t.Fatalf("expected 3 days across leap day, got %d", len(leapBuckets))
	}
	// Day 1: Feb 28 -> Feb 29
	if leapBuckets[0].To.Day() != 29 || leapBuckets[0].To.Month() != time.February {
		t.Errorf("expected leap day bucket ending on Feb 29, got %v", leapBuckets[0].To)
	}
	// Day 2: Feb 29 -> March 1
	if leapBuckets[1].From.Day() != 29 || leapBuckets[1].To.Month() != time.March || leapBuckets[1].To.Day() != 1 {
		t.Errorf("expected leap day bucket from Feb 29 to Mar 1, got [%v, %v)", leapBuckets[1].From, leapBuckets[1].To)
	}
}

func TestCalendarDayBucketDST(t *testing.T) {
	t.Parallel()

	// America/New_York observed DST switch in March 2026 (Spring Forward: March 8, 2026 - 23 hour day)
	nyLoc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("failed to load America/New_York: %v", err)
	}

	b, err := reportcache.NewCalendarDayBucket(nyLoc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 2-day interval covering DST transition: March 7 to March 9, 2026
	from := time.Date(2026, 3, 7, 0, 0, 0, 0, nyLoc)
	to := time.Date(2026, 3, 9, 0, 0, 0, 0, nyLoc)
	iv, _ := reportcache.NewInterval(from, to)

	buckets, err := b.Resolve(iv)
	if err != nil {
		t.Fatalf("Resolve() DST unexpected error: %v", err)
	}

	if len(buckets) != 2 {
		t.Fatalf("expected 2 days across DST transition, got %d", len(buckets))
	}

	// March 7 is normal 24h
	if buckets[0].Duration() != 24*time.Hour {
		t.Errorf("March 7 duration = %v, want 24h", buckets[0].Duration())
	}

	// March 8 is DST Spring Forward, duration is 23h!
	if buckets[1].Duration() != 23*time.Hour {
		t.Errorf("March 8 DST transition duration = %v, want 23h", buckets[1].Duration())
	}

	// Both must start and end at midnight local time
	if buckets[1].From.Hour() != 0 || buckets[1].To.Hour() != 0 {
		t.Errorf("DST bucket must start and end at local midnight: got [%v, %v)", buckets[1].From, buckets[1].To)
	}
}
