package keyboard

import "testing"

func TestKeyEncoding(t *testing.T) {
	cases := []struct {
		name     string
		key      Key
		scan     uint8
		extended bool
		ok       bool
	}{
		{"left ctrl", KeyLeftCtrl, 0x1D, false, true},
		{"right ctrl", KeyRightCtrl, 0x1D, true, true},
		{"right alt", KeyRightAlt, 0x38, true, true},
		{"right shift", KeyRightShift, 0x36, false, true},
		{"arrow up", KeyArrowUp, 0x48, true, true},
		{"numpad 8", KeyNumpad8, 0x48, false, true},
		{"home", KeyHome, 0x47, true, true},
		{"numpad 7", KeyNumpad7, 0x47, false, true},
		{"slash", KeySlash, 0x35, false, true},
		{"numpad divide", KeyNumpadDivide, 0x35, true, true},
		{"divide alias", KeyDivide, 0x35, true, true},
		{"numpad multiply", KeyNumpadMultiply, 0x37, false, true},
		{"left win", KeyLWin, 0x5B, true, true},
		{"right win", KeyRWin, 0x5C, true, true},
		{"numpad enter", KeyNumpadEnter, 0x1C, true, true},
		{"zero scan", Key(0xE000), 0, false, false},
		{"bad prefix", Key(0xE100), 0, false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scan, extended, ok := tc.key.Decode()
			if scan != tc.scan || extended != tc.extended || ok != tc.ok {
				t.Fatalf("Decode() = (%#x, %v, %v), want (%#x, %v, %v)",
					scan, extended, ok, tc.scan, tc.extended, tc.ok)
			}
		})
	}
}

func TestMakeKeyLParam(t *testing.T) {
	cases := []struct {
		name       string
		key        Key
		wantScan   uintptr
		wantExtend bool
	}{
		{"arrow up", KeyArrowUp, 0x48, true},
		{"numpad 8", KeyNumpad8, 0x48, false},
		{"left ctrl", KeyLeftCtrl, 0x1D, false},
		{"right ctrl", KeyRightCtrl, 0x1D, true},
		{"right shift", KeyRightShift, 0x36, false},
		{"numpad multiply", KeyNumpadMultiply, 0x37, false},
		{"slash", KeySlash, 0x35, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scan, extended, ok := tc.key.Decode()
			if !ok {
				t.Fatal("key did not decode")
			}
			lparam := makeKeyLParam(scan, extended, false)
			if got := (lparam >> 16) & 0xFF; got != tc.wantScan {
				t.Errorf("scan bits = %#x, want %#x", got, tc.wantScan)
			}
			if got := lparam&(1<<24) != 0; got != tc.wantExtend {
				t.Errorf("extended bit = %v, want %v", got, tc.wantExtend)
			}
			if up := makeKeyLParam(scan, extended, true); up&(1<<30) == 0 || up&(1<<31) == 0 {
				t.Errorf("key-up lparam lacks previous/transition bits: %#x", up)
			}
		})
	}
}

func TestVirtualKey(t *testing.T) {
	cases := []struct {
		name string
		key  Key
		want uintptr
		ok   bool
	}{
		{"arrow up", KeyArrowUp, 0x26, true},
		{"numpad 8", KeyNumpad8, 0x68, true},
		{"numpad divide", KeyNumpadDivide, 0x6F, true},
		{"numpad multiply", KeyNumpadMultiply, 0x6A, true},
		{"right ctrl type 1", KeyRightCtrl, 0x11, true},
		{"right alt type 1", KeyRightAlt, 0x12, true},
		{"right shift type 1", KeyRightShift, 0x10, true},
		{"left win", KeyLWin, 0x5B, true},
		{"invalid", Key(0xE000), 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := VirtualKey(tc.key)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("VirtualKey() = (%#x, %v), want (%#x, %v)", got, ok, tc.want, tc.ok)
			}
		})
	}
}
