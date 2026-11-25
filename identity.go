package reportcache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// Identity represents the conceptual five-dimension cache identity:
// Namespace + ReportID + ScopeKey + Bucket + QueryVersion.
type Identity struct {
	Namespace    string
	ReportID     string
	ScopeKey     string
	BucketID     string
	Interval     Interval
	QueryVersion string
}

// Validate checks that all five dimensions are provided and non-empty, and Interval is valid.
func (id Identity) Validate() error {
	if strings.TrimSpace(id.Namespace) == "" {
		return ErrEmptyNamespace
	}
	if strings.TrimSpace(id.ReportID) == "" {
		return ErrEmptyReportID
	}
	if strings.TrimSpace(id.ScopeKey) == "" {
		return ErrEmptyScopeKey
	}
	if strings.TrimSpace(id.BucketID) == "" {
		return ErrNilBucketSpec
	}
	if strings.TrimSpace(id.QueryVersion) == "" {
		return ErrEmptyQueryVersion
	}
	if _, err := NewInterval(id.Interval.From, id.Interval.To); err != nil {
		return err
	}
	return nil
}

// StorageKey produces a deterministic, delimiter-collision-free canonical representation of the identity.
// It uses length-prefixed encoding for each component so that characters such as ':', '/', or ';'
// inside Namespace or ScopeKey cannot cause key collisions.
func (id Identity) StorageKey() string {
	return fmt.Sprintf("rc:v1:%d:%s:%d:%s:%d:%s:%d:%s:%s:%d:%s",
		len(id.Namespace), id.Namespace,
		len(id.ReportID), id.ReportID,
		len(id.ScopeKey), id.ScopeKey,
		len(id.BucketID), id.BucketID,
		id.Interval.String(),
		len(id.QueryVersion), id.QueryVersion,
	)
}

// Digest returns the hex-encoded SHA-256 digest of the StorageKey.
func (id Identity) Digest() string {
	sum := sha256.Sum256([]byte(id.StorageKey()))
	return hex.EncodeToString(sum[:])
}
