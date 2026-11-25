package canonical_test

import (
	"errors"
	"testing"

	"github.com/Khan9/reportcache/canonical"
)

func TestValidateNonEmptySlice(t *testing.T) {
	t.Parallel()

	if err := canonical.ValidateNonEmptySlice([]string{}); !errors.Is(err, canonical.ErrEmptyFilterList) {
		t.Errorf("expected ErrEmptyFilterList for empty slice, got: %v", err)
	}

	if err := canonical.ValidateNonEmptySlice[string](nil); err != nil {
		t.Errorf("nil slice should be valid (unrestricted), got: %v", err)
	}

	if err := canonical.ValidateNonEmptySlice([]string{"office-1"}); err != nil {
		t.Errorf("populated slice should be valid, got: %v", err)
	}
}

func TestSet(t *testing.T) {
	t.Parallel()

	// Invariance to ordering and duplicates
	input1 := []string{"office-002", "office-001"}
	input2 := []string{"office-001", "office-002", "office-001"}

	set1 := canonical.Set(input1)
	set2 := canonical.Set(input2)

	if set1 != "office-001,office-002" {
		t.Errorf("Set() = %q, want office-001,office-002", set1)
	}
	if set1 != set2 {
		t.Errorf("Set() must be identical for permutations and duplicates: %q vs %q", set1, set2)
	}
}

func TestFormatKV(t *testing.T) {
	t.Parallel()

	kv := map[string]string{
		"offices":  "001,002",
		"currency": "VND",
		"tenant":   "t1",
	}

	got := canonical.FormatKV(kv)
	expected := "currency=VND;offices=001,002;tenant=t1"

	if got != expected {
		t.Errorf("FormatKV() = %q, want %q", got, expected)
	}
}

func TestHash(t *testing.T) {
	t.Parallel()

	h1 := canonical.Hash("test-string")
	h2 := canonical.Hash("test-string")

	if h1 != h2 {
		t.Errorf("Hash() must be deterministic")
	}
	if len(h1) != 7+64 { // "sha256:" + 64 hex chars
		t.Errorf("Hash() invalid length: %d", len(h1))
	}
}
