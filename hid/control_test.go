package hid

import (
	"testing"

	"github.com/rpdg/winput/hid/interception"
	"github.com/rpdg/winput/keyboard"
	"github.com/rpdg/winput/screen"
)

func TestInitFailureIsNotCached(t *testing.T) {
	initMutex.Lock()
	already := initialized
	initMutex.Unlock()
	if already {
		if err := Init(); err != nil {
			t.Fatal(err)
		}
		return
	}
	interception.SetLibraryPath("winput-missing-interception.dll")
	defer interception.SetLibraryPath("interception.dll")
	if err := Init(); err == nil {
		t.Fatal("missing library initialized")
	}
	initMutex.Lock()
	stuck := initialized
	initMutex.Unlock()
	if stuck {
		t.Fatal("failed init was cached as initialized")
	}
	if err := Init(); err == nil {
		t.Fatal("retry of the missing library initialized")
	}
}

func TestKeyStroke(t *testing.T) {
	cases := []struct {
		name     string
		key      keyboard.Key
		code     uint16
		extended bool
	}{
		{"arrow up", keyboard.KeyArrowUp, 0x48, true},
		{"numpad 8", keyboard.KeyNumpad8, 0x48, false},
		{"left ctrl", keyboard.KeyLeftCtrl, 0x1D, false},
		{"right ctrl", keyboard.KeyRightCtrl, 0x1D, true},
		{"right shift", keyboard.KeyRightShift, 0x36, false},
		{"numpad divide", keyboard.KeyNumpadDivide, 0x35, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, up := range []bool{false, true} {
				stroke, err := keyStroke(uint16(tc.key), up)
				if err != nil {
					t.Fatalf("keyStroke(up=%v): %v", up, err)
				}
				if stroke.Code != tc.code {
					t.Errorf("Code = %#x, want %#x", stroke.Code, tc.code)
				}
				if got := stroke.State&interception.KeyStateE0 != 0; got != tc.extended {
					t.Errorf("E0 = %v, want %v", got, tc.extended)
				}
				wantUp := uint16(interception.KeyStateDown)
				if up {
					wantUp = interception.KeyStateUp
				}
				if stroke.State&^interception.KeyStateE0 != wantUp {
					t.Errorf("state = %#x, want base %#x", stroke.State, wantUp)
				}
			}
		})
	}

	if _, err := keyStroke(uint16(keyboard.Key(0xE000)), false); err == nil {
		t.Fatal("keyStroke accepted zero scan code")
	}
}

func TestHumanArrived(t *testing.T) {
	cases := []struct {
		dx, dy int32
		want   bool
	}{
		{2, 0, true},
		{2, 2, true},
		{1, 1, true},
		{0, 0, true},
		{3, 0, false},
		{0, 3, false},
		{-2, 2, true},
		{-3, 0, false},
	}
	for _, tc := range cases {
		if got := humanArrived(tc.dx, tc.dy); got != tc.want {
			t.Errorf("humanArrived(%d, %d) = %v, want %v", tc.dx, tc.dy, got, tc.want)
		}
	}
}

func TestInsideVirtualBounds(t *testing.T) {
	bounds := screen.Rect{Left: -1920, Top: 0, Right: 1920, Bottom: 1080}
	cases := []struct {
		x, y int32
		want bool
	}{
		{-1920, 0, true},
		{1919, 1079, true},
		{0, 0, true},
		{1920, 0, false},
		{0, 1080, false},
		{-1921, 0, false},
		{0, -1, false},
	}
	for _, tc := range cases {
		if got := insideVirtualBounds(tc.x, tc.y, bounds); got != tc.want {
			t.Errorf("insideVirtualBounds(%d, %d) = %v, want %v", tc.x, tc.y, got, tc.want)
		}
	}
}

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
