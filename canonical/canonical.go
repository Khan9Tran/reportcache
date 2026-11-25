package canonical

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	// ErrEmptyFilterList is returned when a filter slice is non-nil but empty, which represents an invalid request.
	ErrEmptyFilterList = errors.New("canonical: explicit empty filter list is invalid; omit filter to represent all records")
)

// ValidateNonEmptySlice validates that a slice is either nil (representing absent/unrestricted)
// or contains at least one item. It returns ErrEmptyFilterList if items != nil && len(items) == 0.
func ValidateNonEmptySlice[T any](items []T) error {
	if items != nil && len(items) == 0 {
		return ErrEmptyFilterList
	}
	return nil
}

// Set deduplicates and sorts slice elements in lexicographical order.
// If items is empty or nil, it returns empty string.
func Set(items []string) string {
	if len(items) == 0 {
		return ""
	}
	set := make(map[string]struct{}, len(items))
	for _, it := range items {
		set[it] = struct{}{}
	}
	sorted := make([]string, 0, len(set))
	for it := range set {
		sorted = append(sorted, it)
	}
	sort.Strings(sorted)
	return strings.Join(sorted, ",")
}

// FormatKV formats key-value pairs deterministically by sorting keys in lexicographical order.
// Delimiters within keys or values are not stripped; for arbitrary input consider Hash.
func FormatKV(kv map[string]string) string {
	if len(kv) == 0 {
		return ""
	}
	keys := make([]string, 0, len(kv))
	for k := range kv {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(';')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(kv[k])
	}
	return b.String()
}

// Hash returns the hex-encoded SHA-256 digest of s with prefix "sha256:".
func Hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("sha256:%s", hex.EncodeToString(sum[:]))
}
