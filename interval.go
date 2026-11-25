package reportcache

import (
	"fmt"
	"time"
)

// Interval represents a half-open time interval [From, To).
// The start instant From is inclusive; the end instant To is exclusive.
type Interval struct {
	From time.Time
	To   time.Time
}

// NewInterval creates and validates a half-open interval [from, to).
// It returns ErrInvalidInterval if either time is zero or if from is not strictly before to.
func NewInterval(from, to time.Time) (Interval, error) {
	if from.IsZero() || to.IsZero() || !from.Before(to) {
		return Interval{}, ErrInvalidInterval
	}
	return Interval{From: from, To: to}, nil
}

// Duration returns the duration of the interval.
func (i Interval) Duration() time.Duration {
	return i.To.Sub(i.From)
}

// Contains returns true if instant t falls within the half-open interval [From, To).
func (i Interval) Contains(t time.Time) bool {
	return (t.Equal(i.From) || t.After(i.From)) && t.Before(i.To)
}

// Overlaps returns true if this interval and another interval have any common instants.
// For two half-open intervals [A_from, A_to) and [B_from, B_to), they overlap if and only if
// A_from < B_to and B_from < A_to.
func (i Interval) Overlaps(other Interval) bool {
	return i.From.Before(other.To) && other.From.Before(i.To)
}

// IsAdjacent returns true if this interval directly touches the other interval without overlap or gap.
// That is, i.To == other.From or i.From == other.To.
func (i Interval) IsAdjacent(other Interval) bool {
	return i.To.Equal(other.From) || i.From.Equal(other.To)
}

// String returns an ISO8601-like half-open interval representation "[From, To)".
func (i Interval) String() string {
	return fmt.Sprintf("[%s, %s)", i.From.Format(time.RFC3339Nano), i.To.Format(time.RFC3339Nano))
}
