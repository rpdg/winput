package mouse

import (
	"errors"
	"testing"
)

func TestWheelDelta(t *testing.T) {
	if WHEEL_DELTA != 120 {
		t.Fatalf("WHEEL_DELTA = %d, want 120", WHEEL_DELTA)
	}
	for _, delta := range []int32{32880, -32880, 32768, -32769} {
		if _, err := wheelDelta(delta); err == nil {
			t.Errorf("wheelDelta(%d) accepted out-of-range delta", delta)
		}
	}
	for _, delta := range []int32{32767, -32768, 120, -120, 60, 30, 0} {
		got, err := wheelDelta(delta)
		if err != nil {
			t.Errorf("wheelDelta(%d): %v", delta, err)
			continue
		}
		if int32(got) != delta {
			t.Errorf("wheelDelta(%d) = %d", delta, got)
		}
	}
}

func TestMakeLParamRange(t *testing.T) {
	if _, err := makeLParam(32768, 0); err == nil {
		t.Fatal("makeLParam accepted x=32768")
	}
	if _, err := makeLParam(0, -32769); err == nil {
		t.Fatal("makeLParam accepted y=-32769")
	}
	if _, err := makeLParam(-32769, 0); !errors.Is(err, ErrInvalidMouseCoordinate) {
		t.Fatalf("makeLParam(x=-32769) = %v, want ErrInvalidMouseCoordinate", err)
	}
	got, err := makeLParam(32767, -32768)
	if err != nil {
		t.Fatalf("makeLParam(edge) = %v", err)
	}
	// Low word is 32767. High word is the two's-complement of -32768, which is 0x8000.
	const want = uintptr(32767) | uintptr(0x8000)<<16
	if got != want {
		t.Fatalf("makeLParam(edge) = %#x, want %#x", got, want)
	}
}

func TestDoubleClickSequence(t *testing.T) {
	want := [4]buttonMessage{
		{WM_LBUTTONDOWN, MK_LBUTTON, false},
		{WM_LBUTTONUP, 0, true},
		{WM_LBUTTONDBLCLK, MK_LBUTTON, false},
		{WM_LBUTTONUP, 0, true},
	}
	got := doubleClickSequence()
	if got != want {
		t.Fatalf("doubleClickSequence() = %+v, want %+v", got, want)
	}
}
