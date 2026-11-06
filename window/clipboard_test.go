package window

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf16"
)

// encodeToSame returns the expected encoding of s (UTF-16 code units plus the
// terminating NUL) using utf16.Encode as the reference encoder.
func encodeToSame(s string) []uint16 {
	units := utf16.Encode([]rune(s))
	return append(units, 0)
}

// TestEncodeUnicodeText covers newline normalization, a lone "\r", a "\r\n"
// pair, non-BMP surrogate encoding, and the oversized-input rejection. It
// never touches the real clipboard.
func TestEncodeUnicodeText(t *testing.T) {
	cases := []struct {
		name      string
		text      string
		normalize bool
		want      []uint16
		wantErr   bool
	}{
		{"lone lf normalized", "a\nb", true, encodeToSame("a\r\nb"), false},
		{"crlf stays", "a\r\nb", true, encodeToSame("a\r\nb"), false},
		{"lone cr stays", "a\rb", true, encodeToSame("a\rb"), false},
		{"cr cr lf stays", "a\r\r\nb", true, encodeToSame("a\r\r\nb"), false},
		{"lf lf each becomes crlf", "a\n\nb", true, encodeToSame("a\r\n\r\nb"), false},
		{"normalize false keeps lf", "a\nb", false, encodeToSame("a\nb"), false},
		{"normalize false keeps lone cr", "a\rb", false, encodeToSame("a\rb"), false},
		{"empty is just nul", "", true, []uint16{0}, false},
		{"bmp passthrough", "Hi 你", true, encodeToSame("Hi 你"), false},
		{"non-bmp surrogate pair", "a😀b", true, encodeToSame("a😀b"), false},
		{"oversized rejected", strings.Repeat("a", maxClipboardBytes/2), true, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := encodeUnicodeText(tc.text, tc.normalize)
			if tc.wantErr {
				if err == nil {
					t.Fatal("oversized input encoded without error")
				}
				if !errors.Is(err, ErrClipboardFailed) {
					t.Fatalf("oversized error = %v, want ErrClipboardFailed", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("encodeUnicodeText: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("unit count = %d, want %d", len(got), len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("unit %d = %#x, want %#x", i, got[i], tc.want[i])
				}
			}
			if got[len(got)-1] != 0 {
				t.Fatalf("last unit = %#x, want terminating NUL", got[len(got)-1])
			}
		})
	}
}

// TestCountUnicodeCodeUnits pins the counting pass that runs before any
// allocation, including the boundary just below the internal limit.
func TestCountUnicodeCodeUnits(t *testing.T) {
	cases := []struct {
		text string
		norm bool
		want int
	}{
		{"a\nb", true, 4},   // a + \r\n + b
		{"a\nb", false, 3},  // a, \n, b
		{"a\r\nb", true, 4}, // a, \r, \n, b
		{"A😀", true, 3},     // A + surrogate pair
		{"", true, 0},
	}
	for _, tc := range cases {
		if got := countUnicodeCodeUnits(tc.text, tc.norm); got != tc.want {
			t.Errorf("countUnicodeCodeUnits(%q, %v) = %d, want %d", tc.text, tc.norm, got, tc.want)
		}
	}
	// A text of exactly maxClipboardBytes/2 code units is allowed; one more
	// unit is rejected. Only the counting path is exercised here (no buffer).
	if got := countUnicodeCodeUnits(strings.Repeat("a", maxClipboardBytes/2-1), true); got != maxClipboardBytes/2-1 {
		t.Fatalf("boundary count = %d", got)
	}
}
