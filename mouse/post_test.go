package mouse

import "testing"

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
