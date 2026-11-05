package winput

import (
	"errors"
	"os"
	"testing"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/rpdg/winput/hid"
	"github.com/rpdg/winput/keyboard"
	"github.com/rpdg/winput/screen"
)

func TestInputLayout(t *testing.T) {
	ptrSize := unsafe.Sizeof(uintptr(0))
	var wantSize, wantBase, wantExtra uintptr
	if ptrSize == 8 {
		wantSize, wantBase, wantExtra = 40, 8, 16
	} else {
		wantSize, wantBase, wantExtra = 28, 4, 12
	}

	if got := unsafe.Sizeof(input{}); got != wantSize {
		t.Fatalf("sizeof(input) = %d, want %d", got, wantSize)
	}
	if got := unsafe.Offsetof(input{}.Mi); got != wantBase {
		t.Fatalf("offset of union = %d, want %d", got, wantBase)
	}
	wantFieldOffsets := []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"wVk", unsafe.Offsetof(keyboardInput{}.WVk), 0},
		{"wScan", unsafe.Offsetof(keyboardInput{}.WScan), 2},
		{"dwFlags", unsafe.Offsetof(keyboardInput{}.DwFlags), 4},
		{"time", unsafe.Offsetof(keyboardInput{}.Time), 8},
		{"dwExtraInfo", unsafe.Offsetof(keyboardInput{}.DwExtraInfo), wantExtra},
	}
	for _, f := range wantFieldOffsets {
		if f.got != f.want {
			t.Errorf("%s offset = %d, want %d", f.name, f.got, f.want)
		}
	}
}

func TestScanCodeInput(t *testing.T) {
	in, ok := scanCodeInput(KeyRightCtrl, false)
	if !ok {
		t.Fatal("right ctrl did not build")
	}
	ki := (*keyboardInput)(unsafe.Pointer(&in.Mi))
	if ki.WVk != 0 || ki.WScan != 0x1D {
		t.Fatalf("right ctrl vk/scan = %#x/%#x, want 0/0x1D", ki.WVk, ki.WScan)
	}
	if ki.DwFlags&KEYEVENTF_SCANCODE == 0 || ki.DwFlags&KEYEVENTF_EXTENDEDKEY == 0 {
		t.Fatalf("right ctrl flags = %#x, want scancode and extended", ki.DwFlags)
	}
	if up, ok := scanCodeInput(KeyRightCtrl, true); !ok {
		t.Fatal("right ctrl up did not build")
	} else {
		kiUp := (*keyboardInput)(unsafe.Pointer(&up.Mi))
		if kiUp.DwFlags&KEYEVENTF_KEYUP == 0 {
			t.Fatalf("right ctrl up flags = %#x, want KEYUP", kiUp.DwFlags)
		}
	}
	if _, ok := scanCodeInput(Key(0xE000), false); ok {
		t.Fatal("zero scan code built an input")
	}
	if err := sendScanCode(Key(0xE000), false); !errors.Is(err, ErrUnsupportedKey) {
		t.Fatalf("sendScanCode(invalid) = %v, want ErrUnsupportedKey", err)
	}
}

func TestErrorAliases(t *testing.T) {
	if ErrUnsupportedKey != keyboard.ErrUnsupportedKey {
		t.Fatal("ErrUnsupportedKey is not the keyboard sentinel")
	}
	if ErrMoveNotConverged != hid.ErrMoveNotConverged {
		t.Fatal("ErrMoveNotConverged is not the hid sentinel")
	}
	if ErrOutsideVirtualDesktop != hid.ErrOutsideVirtualDesktop {
		t.Fatal("ErrOutsideVirtualDesktop is not the hid sentinel")
	}
}

func TestWithinVirtualDesktop(t *testing.T) {
	bounds := screen.Rect{Left: 0, Top: 0, Right: 1920, Bottom: 1080}
	if !withinVirtualDesktop(1919, 1079, bounds) {
		t.Fatal("last pixel rejected")
	}
	if withinVirtualDesktop(1920, 0, bounds) || withinVirtualDesktop(0, 1080, bounds) {
		t.Fatal("half-open right/bottom edge accepted")
	}
}

func TestUnicodeInputs(t *testing.T) {
	if got := unicodeInputs('A'); len(got) != 2 {
		t.Fatalf("BMP input count = %d, want 2", len(got))
	}
	bmp := unicodeInputs('A')
	for i, in := range bmp {
		ki := (*keyboardInput)(unsafe.Pointer(&in.Mi))
		if in.Type != INPUT_KEYBOARD || ki.WVk != 0 || ki.WScan != 'A' {
			t.Errorf("BMP input %d has wrong fields: type=%d vk=%d scan=%#x", i, in.Type, ki.WVk, ki.WScan)
		}
		if ki.DwFlags&KEYEVENTF_UNICODE == 0 {
			t.Errorf("BMP input %d lacks KEYEVENTF_UNICODE", i)
		}
		wantUp := i == 1
		if gotUp := ki.DwFlags&KEYEVENTF_KEYUP != 0; gotUp != wantUp {
			t.Errorf("BMP input %d key-up = %v, want %v", i, gotUp, wantUp)
		}
	}

	const nonBMP = rune(0x1F600)
	got := unicodeInputs(nonBMP)
	if len(got) != 4 {
		t.Fatalf("non-BMP input count = %d, want 4", len(got))
	}
	units := utf16.Encode([]rune{nonBMP})
	for i, in := range got {
		ki := (*keyboardInput)(unsafe.Pointer(&in.Mi))
		wantUnit := units[i/2]
		if ki.WScan != wantUnit {
			t.Errorf("input %d scan = %#x, want %#x", i, ki.WScan, wantUnit)
		}
		wantUp := i%2 == 1
		if gotUp := ki.DwFlags&KEYEVENTF_KEYUP != 0; gotUp != wantUp {
			t.Errorf("input %d key-up = %v, want %v", i, gotUp, wantUp)
		}
	}
}

func TestInputScope(t *testing.T) {
	t.Run("second down failure only releases the first", func(t *testing.T) {
		var firstUp, secondUp int
		downErr := errors.New("second down failed")
		var scope inputScope
		if err := scope.Press(
			func() error { return nil },
			func() error { firstUp++; return nil },
		); err != nil {
			t.Fatal(err)
		}
		err := scope.Press(
			func() error { return downErr },
			func() error { secondUp++; return nil },
		)
		if !errors.Is(err, downErr) {
			t.Fatalf("Press returned %v, want %v", err, downErr)
		}
		if err := scope.Release(); err != nil {
			t.Fatal(err)
		}
		if firstUp != 1 || secondUp != 0 {
			t.Fatalf("up calls: first=%d second=%d, want 1/0", firstUp, secondUp)
		}
	})

	t.Run("failed release is retried and remaining releases still run", func(t *testing.T) {
		var firstUp, secondUp int
		releaseErr := errors.New("second up failed")
		var scope inputScope
		if err := scope.Press(func() error { return nil }, func() error { firstUp++; return nil }); err != nil {
			t.Fatal(err)
		}
		if err := scope.Press(func() error { return nil }, func() error {
			secondUp++
			return releaseErr
		}); err != nil {
			t.Fatal(err)
		}
		err := scope.Release()
		if !errors.Is(err, releaseErr) {
			t.Fatalf("Release returned %v, want %v", err, releaseErr)
		}
		if firstUp != 1 || secondUp != 2 {
			t.Fatalf("up calls: first=%d second=%d, want 1/2", firstUp, secondUp)
		}
	})
}

func TestHotkeyNeedsForeground(t *testing.T) {
	cases := []struct {
		name string
		keys []Key
		want bool
	}{
		{"right ctrl", []Key{KeyRightCtrl, KeyA}, true},
		{"right alt", []Key{KeyRightAlt, KeyA}, true},
		{"right shift", []Key{KeyRightShift, KeyA}, true},
		{"left win", []Key{KeyLWin, KeyA}, false},
		{"right win", []Key{KeyRWin, KeyA}, false},
		{"plain key", []Key{KeyA}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hotkeyNeedsForeground(tc.keys); got != tc.want {
				t.Fatalf("hotkeyNeedsForeground() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMouseButtonMapping(t *testing.T) {
	if MouseLeft != 1 || MouseRight != 2 || MouseMiddle != 3 {
		t.Fatalf("MouseButton identities = %d/%d/%d", MouseLeft, MouseRight, MouseMiddle)
	}
	if flags, ok := mouseEventButtonFlags(MouseLeft, true); !ok || flags != 0x0002 {
		t.Fatalf("left down flags = %#x, %v; want 0x0002, true", flags, ok)
	}
	if flags, ok := mouseEventButtonFlags(MouseLeft, false); !ok || flags != 0x0004 {
		t.Fatalf("left up flags = %#x, %v; want 0x0004, true", flags, ok)
	}
	down, up, ok := hidButtonStates(MouseLeft)
	if !ok || down != 0x001 || up != 0x002 {
		t.Fatalf("left HID states = %#x/%#x, %v; want 0x001/0x002, true", down, up, ok)
	}
	if _, _, ok := hidButtonStates(MouseButton(4)); ok {
		t.Fatal("invalid button accepted")
	}
	if _, ok := mouseEventButtonFlags(MouseButton(4), true); ok {
		t.Fatal("invalid button accepted by message mapping")
	}
}

func TestScrollWheelValidation(t *testing.T) {
	for _, steps := range []int{0, 274, -274, 536870913} {
		if err := ScrollWheel(steps); !errors.Is(err, ErrInvalidScrollSteps) {
			t.Errorf("ScrollWheel(%d) = %v, want ErrInvalidScrollSteps", steps, err)
		}
	}
}

func TestPressForRejectsShortDuration(t *testing.T) {
	for _, d := range []time.Duration{9 * time.Millisecond, 0, -time.Millisecond} {
		if err := PressFor(KeyA, d); !errors.Is(err, ErrInvalidPressDuration) {
			t.Errorf("PressFor(%v) = %v, want ErrInvalidPressDuration", d, err)
		}
	}
}

func TestExactMoveRejectsRightEdge(t *testing.T) {
	bounds := screen.VirtualBounds()
	if err := MoveMouseToExact(bounds.Right, bounds.Top); !errors.Is(err, ErrOutsideVirtualDesktop) {
		t.Fatalf("MoveMouseToExact(right edge) = %v, want ErrOutsideVirtualDesktop", err)
	}
}

func TestMoveMouseByOverflow(t *testing.T) {
	const maxInt32 = 1<<31 - 1
	const minInt32 = -1 << 31
	cases := []struct {
		name           string
		cx, cy, dx, dy int32
		wantX, wantY   int32
		ok             bool
	}{
		{"fits plus x", 0, 0, maxInt32, 0, maxInt32, 0, true},
		{"fits minus x", 0, 0, minInt32, 0, minInt32, 0, true},
		{"overflow x", 1, 0, maxInt32, 0, 0, 0, false},
		{"overflow y", 0, 1, 0, maxInt32, 0, 0, false},
		{"underflow x", -1, 0, minInt32, 0, 0, 0, false},
		{"underflow y", 0, -1, 0, minInt32, 0, 0, false},
		{"normal delta", 5, -7, 10, -3, 15, -10, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			x, y, ok := addCursorDelta(tc.cx, tc.cy, tc.dx, tc.dy)
			if ok != tc.ok || x != tc.wantX || y != tc.wantY {
				t.Fatalf("addCursorDelta() = (%d, %d, %v), want (%d, %d, %v)",
					x, y, ok, tc.wantX, tc.wantY, tc.ok)
			}
		})
	}
}

func TestSendUnicodeIntegration(t *testing.T) {
	if os.Getenv("WINPUT_SENDINPUT_TEST") == "" {
		t.Skip("set WINPUT_SENDINPUT_TEST=1 to inject one BMP character")
	}
	if err := sendUnicode('A'); err != nil {
		t.Fatalf("sendUnicode('A'): %v", err)
	}
}
