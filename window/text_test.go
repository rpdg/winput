package window

import (
	"errors"
	"testing"
)

func TestWindowTextChars(t *testing.T) {
	if n, err := windowTextChars(0); err != nil || n != 0 {
		t.Fatalf("windowTextChars(0) = %d, %v", n, err)
	}
	maxOK := uintptr(maxClipboardBytes/2 - 1)
	if n, err := windowTextChars(maxOK); err != nil || n != int(maxOK) {
		t.Fatalf("windowTextChars(max-1) = %d, %v", n, err)
	}
	for _, n := range []uintptr{maxClipboardBytes / 2, ^uintptr(0)} {
		_, err := windowTextChars(n)
		if !errors.Is(err, ErrReadTextFailed) {
			t.Fatalf("windowTextChars(%d) = %v, want ErrReadTextFailed", n, err)
		}
	}
}
