package reportcache

import "errors"

var (
	// ErrInvalidInterval is returned when From is not strictly before To, or a time is zero.
	ErrInvalidInterval = errors.New("reportcache: invalid interval: from must be strictly before to")

	// ErrUnalignedInterval is returned when an interval does not align with bucket boundaries.
	ErrUnalignedInterval = errors.New("reportcache: interval is not aligned with bucket boundaries")

	// ErrEmptyNamespace is returned when Namespace is empty.
	ErrEmptyNamespace = errors.New("reportcache: namespace must not be empty")

	// ErrEmptyReportID is returned when ReportID is empty.
	ErrEmptyReportID = errors.New("reportcache: report ID must not be empty")

	// ErrEmptyScopeKey is returned when ScopeKey is empty.
	ErrEmptyScopeKey = errors.New("reportcache: scope key must not be empty")

	// ErrEmptyQueryVersion is returned when QueryVersion is empty.
	ErrEmptyQueryVersion = errors.New("reportcache: query version must not be empty")

	// ErrNilBucketSpec is returned when BucketSpec is nil or bucket ID is empty.
	ErrNilBucketSpec = errors.New("reportcache: bucket specification must not be nil")

	// ErrInvalidBucketDuration is returned when a fixed bucket duration is non-positive.
	ErrInvalidBucketDuration = errors.New("reportcache: fixed bucket duration must be positive")

	// ErrNilLocation is returned when a calendar bucket timezone location is nil.
	ErrNilLocation = errors.New("reportcache: timezone location must not be nil")
)
