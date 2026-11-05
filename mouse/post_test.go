package mouse

import "testing"

func TestWheelDelta(t *testing.T) {
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
		{WM_LBUTTONDOWN, MK_LBUTTON},
		{WM_LBUTTONUP, 0},
		{WM_LBUTTONDBLCLK, MK_LBUTTON},
		{WM_LBUTTONUP, 0},
	}
	got := doubleClickSequence()
	if got != want {
		t.Fatalf("doubleClickSequence() = %+v, want %+v", got, want)
	}
}
