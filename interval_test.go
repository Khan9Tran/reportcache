package reportcache_test

import (
	"testing"
	"time"

	"github.com/Khan9Tran/reportcache"
)

func TestNewInterval(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		from    time.Time
		to      time.Time
		wantErr bool
	}{
		{
			name:    "valid interval",
			from:    now,
			to:      now.Add(24 * time.Hour),
			wantErr: false,
		},
		{
			name:    "zero from",
			from:    time.Time{},
			to:      now,
			wantErr: true,
		},
		{
			name:    "zero to",
			from:    now,
			to:      time.Time{},
			wantErr: true,
		},
		{
			name:    "from equals to (empty interval)",
			from:    now,
			to:      now,
			wantErr: true,
		},
		{
			name:    "from after to (inverted interval)",
			from:    now.Add(time.Hour),
			to:      now,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			iv, err := reportcache.NewInterval(tt.from, tt.to)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewInterval() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if !iv.From.Equal(tt.from) || !iv.To.Equal(tt.to) {
					t.Errorf("NewInterval() got From=%v, To=%v; want From=%v, To=%v", iv.From, iv.To, tt.from, tt.to)
				}
				if iv.Duration() != tt.to.Sub(tt.from) {
					t.Errorf("Duration() = %v, want %v", iv.Duration(), tt.to.Sub(tt.from))
				}
			}
		})
	}
}

func TestIntervalContains(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	iv, err := reportcache.NewInterval(from, to)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name string
		t    time.Time
		want bool
	}{
		{"before From", from.Add(-time.Nanosecond), false},
		{"at From (inclusive boundary)", from, true},
		{"inside interval", from.Add(12 * time.Hour), true},
		{"1ns before To", to.Add(-time.Nanosecond), true},
		{"at To (exclusive boundary)", to, false},
		{"after To", to.Add(time.Nanosecond), false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := iv.Contains(tt.t)
			if got != tt.want {
				t.Errorf("Contains(%v) = %v, want %v", tt.t, got, tt.want)
			}
		})
	}
}

func TestIntervalOverlapsAndAdjacent(t *testing.T) {
	t.Parallel()

	t0 := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)

	iv12, _ := reportcache.NewInterval(t1, t2)
	iv01, _ := reportcache.NewInterval(t0, t1)
	iv23, _ := reportcache.NewInterval(t2, t3)
	iv02, _ := reportcache.NewInterval(t0, t2)
	iv13, _ := reportcache.NewInterval(t1, t3)
	iv03, _ := reportcache.NewInterval(t0, t3)

	// Adjacency tests
	if !iv12.IsAdjacent(iv01) {
		t.Errorf("expected iv12 to be adjacent to iv01")
	}
	if !iv12.IsAdjacent(iv23) {
		t.Errorf("expected iv12 to be adjacent to iv23")
	}
	if iv12.IsAdjacent(iv03) {
		t.Errorf("expected iv12 NOT to be adjacent to iv03")
	}

	// Overlap tests
	if iv12.Overlaps(iv01) {
		t.Errorf("adjacent intervals must NOT overlap: iv12 overlaps iv01")
	}
	if iv12.Overlaps(iv23) {
		t.Errorf("adjacent intervals must NOT overlap: iv12 overlaps iv23")
	}
	if !iv12.Overlaps(iv02) {
		t.Errorf("iv12 must overlap iv02")
	}
	if !iv12.Overlaps(iv13) {
		t.Errorf("iv12 must overlap iv13")
	}
	if !iv12.Overlaps(iv03) {
		t.Errorf("iv12 must overlap enclosing iv03")
	}
}
