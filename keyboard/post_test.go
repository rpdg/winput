package keyboard

import (
	"errors"
	"testing"
	"time"
)

// TestTypeWithIntervalRejectsNegative verifies that a negative gap fails
// before any WM_CHAR is posted: the validation happens at the top of
// TypeWithInterval, so no window handle is needed to reach the sentinel.
func TestTypeWithIntervalRejectsNegative(t *testing.T) {
	if err := TypeWithInterval(0, "abc", -time.Millisecond); !errors.Is(err, ErrInvalidInterval) {
		t.Fatalf("TypeWithInterval(negative) = %v, want ErrInvalidInterval", err)
	}
}

// TestTypeEmptyNoPost verifies that empty input sends nothing and returns
// nil, and that a negative gap is still rejected for empty input.
func TestTypeEmptyNoPost(t *testing.T) {
	if err := TypeWithInterval(0, "", -time.Millisecond); !errors.Is(err, ErrInvalidInterval) {
		t.Fatalf("TypeWithInterval(negative, empty) = %v, want ErrInvalidInterval", err)
	}
	for _, gap := range []time.Duration{0, DefaultTypeGap} {
		if err := TypeWithInterval(0, "", gap); err != nil {
			t.Fatalf("TypeWithInterval(gap=%v, empty) = %v, want nil", gap, err)
		}
	}
	if err := Type(0, ""); err != nil {
		t.Fatalf("Type(empty) = %v, want nil", err)
	}
}

// TestDefaultTypeGapIsSeparateFromHold pins the default character gap value
// so the fallback used by winput's Type stays in sync with the documented
// behavior. It is deliberately not the same constant as winput's press hold.
func TestDefaultTypeGapIsSeparateFromHold(t *testing.T) {
	if DefaultTypeGap != 30*time.Millisecond {
		t.Fatalf("DefaultTypeGap = %v, want 30ms", DefaultTypeGap)
	}
}
