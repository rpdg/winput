package hid

import (
	"errors"
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

type mockSlot struct {
	hwids []string
	err   error
}

type mockLifecycle struct {
	loadErr      error
	ctxToReturn  interception.Context
	destroyCalls []interception.Context
	unloadCalls  int
	slots        map[interception.Device]mockSlot
}

func setupMockHID(t *testing.T, m *mockLifecycle) {
	t.Helper()

	origLoad := interceptionLoad
	origUnload := interceptionUnload
	origCreate := interceptionCreateContext
	origDestroy := interceptionDestroyContext
	origScanner := defaultScanner

	initMutex.Lock()
	origCtx := ctx
	origMouse := mouseDev
	origKbd := keyboardDev
	origInit := initialized
	ctx = 0
	mouseDev = 0
	keyboardDev = 0
	initialized = false
	initMutex.Unlock()

	interceptionLoad = func() error { return m.loadErr }
	interceptionUnload = func() { m.unloadCalls++ }
	interceptionCreateContext = func() interception.Context {
		if m.ctxToReturn == 0 {
			return interception.Context(0x100)
		}
		return m.ctxToReturn
	}
	interceptionDestroyContext = func(c interception.Context) {
		m.destroyCalls = append(m.destroyCalls, c)
	}
	defaultScanner = deviceScanner{
		isKeyboard: func(dev interception.Device) bool {
			return dev >= 1 && dev <= 10
		},
		isMouse: func(dev interception.Device) bool {
			return dev >= 11 && dev <= 20
		},
		getHardwareID: func(c interception.Context, dev interception.Device) ([]string, error) {
			slot, ok := m.slots[dev]
			if !ok {
				return nil, nil
			}
			return slot.hwids, slot.err
		},
	}

	t.Cleanup(func() {
		interceptionLoad = origLoad
		interceptionUnload = origUnload
		interceptionCreateContext = origCreate
		interceptionDestroyContext = origDestroy
		defaultScanner = origScanner

		initMutex.Lock()
		ctx = origCtx
		mouseDev = origMouse
		keyboardDev = origKbd
		initialized = origInit
		initMutex.Unlock()
	})
}

func assertHIDState(t *testing.T, wantInit bool, wantMouse, wantKbd interception.Device) {
	t.Helper()
	initMutex.Lock()
	defer initMutex.Unlock()
	if initialized != wantInit || mouseDev != wantMouse || keyboardDev != wantKbd {
		t.Fatalf("HID state = (init=%v, mouse=%d, kbd=%d), want (init=%v, mouse=%d, kbd=%d)",
			initialized, mouseDev, keyboardDev, wantInit, wantMouse, wantKbd)
	}
}

func TestIsVirtualDDHardwareID(t *testing.T) {
	cases := []struct {
		name  string
		hwids []string
		want  bool
	}{
		{
			name:  "DD keyboard collection mixed case",
			hwids: []string{`Hid\DdHid63340&Col07`, `HID\VID_9AC5&UP:0001_U:0006`},
			want:  true,
		},
		{
			name:  "DD parent enumerator uppercase",
			hwids: []string{`DDHID63340\HID_DEVICE`},
			want:  true,
		},
		{
			name:  "DD positioned later in MULTI_SZ",
			hwids: []string{`HID_DEVICE`, `HID_DEVICE_SYSTEM_MOUSE`, `HID\ddhid63340&Col03`},
			want:  true,
		},
		{
			name:  "exact token without suffix",
			hwids: []string{`hid\ddhid63340`},
			want:  true,
		},
		{
			name:  "physical Logitech keyboard",
			hwids: []string{`HID\VID_046D&PID_C31C&REV_4920&MI_00`, `HID_DEVICE_SYSTEM_KEYBOARD`},
			want:  false,
		},
		{
			name:  "physical ACPI keyboard",
			hwids: []string{`ACPI\VEN_LEN&DEV_0071`, `ACPI\LEN0071`, `*LEN0071`},
			want:  false,
		},
		{
			name:  "no false positive on 04DD or 6334 substring",
			hwids: []string{`HID\VID_04DD&PID_6334&MI_00`, `HID\VID_06CB&PID_63340`},
			want:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isVirtualDDHardwareID(tc.hwids); got != tc.want {
				t.Errorf("isVirtualDDHardwareID(%#v) = %v, want %v", tc.hwids, got, tc.want)
			}
		})
	}
}

func TestDeviceSelectionAndRejection(t *testing.T) {
	t.Run("U1 desktop without PS2 skips leading DD before USB", func(t *testing.T) {
		m := &mockLifecycle{
			slots: map[interception.Device]mockSlot{
				1:  {hwids: []string{`HID\ddhid63340&Col07`, `HID_DEVICE_SYSTEM_KEYBOARD`}},
				2:  {hwids: []string{`HID\VID_046D&PID_C31C&REV_4920&MI_00`}},
				11: {hwids: []string{`HID\ddhid63340&Col03`, `HID_DEVICE_SYSTEM_MOUSE`}},
				12: {hwids: []string{`HID\ddhid63340&Col04`, `HID_DEVICE_SYSTEM_MOUSE`}},
				13: {hwids: []string{`HID\VID_046D&PID_C52F&REV_3000&MI_00`}},
			},
		}
		setupMockHID(t, m)

		if err := Init(); err != nil {
			t.Fatalf("Init() = %v", err)
		}
		assertHIDState(t, true, 13, 2)
	})

	t.Run("U2 multi-physical keeps first valid non-DD device", func(t *testing.T) {
		m := &mockLifecycle{
			slots: map[interception.Device]mockSlot{
				1:  {hwids: []string{`ACPI\VEN_LEN&DEV_0071`}},
				2:  {hwids: []string{`HID\ddhid63340&Col07`}},
				4:  {hwids: []string{`HID\VID_046D&PID_C31C&REV_4920&MI_00`}},
				11: {hwids: []string{`ACPI\VEN_LEN&DEV_2046`}},
				12: {hwids: []string{`HID\VID_06CB&PID_000F&Col01`}},
				13: {hwids: []string{`HID\ddhid63340&Col03`}},
				17: {hwids: []string{`HID\VID_046D&PID_C52F&REV_3000&MI_00`}},
			},
		}
		setupMockHID(t, m)

		if err := Init(); err != nil {
			t.Fatalf("Init() = %v", err)
		}
		assertHIDState(t, true, 11, 1)
	})

	t.Run("U3 interleaved empty slots skipped before physical USB", func(t *testing.T) {
		m := &mockLifecycle{
			slots: map[interception.Device]mockSlot{
				2:  {hwids: []string{`HID\ddhid63340&Col07`}},
				4:  {hwids: []string{`HID\VID_046D&PID_C31C&MI_00`}},
				15: {hwids: []string{`HID\VID_046D&PID_C52F&MI_00`}},
			},
		}
		setupMockHID(t, m)

		if err := Init(); err != nil {
			t.Fatalf("Init() = %v", err)
		}
		assertHIDState(t, true, 15, 4)
	})

	t.Run("U4 all DD rejected cleans up and allows retry after USB plug-in", func(t *testing.T) {
		m := &mockLifecycle{
			slots: map[interception.Device]mockSlot{
				1:  {hwids: []string{`HID\ddhid63340&Col07`}},
				11: {hwids: []string{`HID\ddhid63340&Col03`}},
				12: {hwids: []string{`HID\ddhid63340&Col04`}},
			},
		}
		setupMockHID(t, m)

		err := Init()
		if !errors.Is(err, ErrDevicesUnavailable) {
			t.Fatalf("Init() = %v, want wrapped ErrDevicesUnavailable", err)
		}
		if len(m.destroyCalls) != 1 || m.unloadCalls != 1 {
			t.Fatalf("cleanup = (destroy=%d, unload=%d), want (1, 1)", len(m.destroyCalls), m.unloadCalls)
		}
		assertHIDState(t, false, 0, 0)

		m.slots[2] = mockSlot{hwids: []string{`HID\VID_046D&PID_C31C&MI_00`}}
		m.slots[13] = mockSlot{hwids: []string{`HID\VID_046D&PID_C52F&MI_00`}}
		if err := Init(); err != nil {
			t.Fatalf("retry Init() = %v", err)
		}
		assertHIDState(t, true, 13, 2)
	})

	t.Run("U5 single class DD-only fails closed at Init", func(t *testing.T) {
		cases := []struct {
			name  string
			slots map[interception.Device]mockSlot
		}{
			{
				name: "physical keyboard with DD-only mouse",
				slots: map[interception.Device]mockSlot{
					1:  {hwids: []string{`ACPI\VEN_LEN&DEV_0071`}},
					11: {hwids: []string{`HID\ddhid63340&Col03`}},
					12: {hwids: []string{`HID\ddhid63340&Col04`}},
				},
			},
			{
				name: "DD-only keyboard with physical mouse",
				slots: map[interception.Device]mockSlot{
					1:  {hwids: []string{`HID\ddhid63340&Col07`}},
					11: {hwids: []string{`HID\VID_046D&PID_C52F&MI_00`}},
				},
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				m := &mockLifecycle{slots: tc.slots}
				setupMockHID(t, m)

				err := Init()
				if !errors.Is(err, ErrDevicesUnavailable) {
					t.Fatalf("Init() = %v, want wrapped ErrDevicesUnavailable", err)
				}
				if len(m.destroyCalls) != 1 || m.unloadCalls != 1 {
					t.Fatalf("cleanup = (destroy=%d, unload=%d), want (1, 1)", len(m.destroyCalls), m.unloadCalls)
				}
				assertHIDState(t, false, 0, 0)
			})
		}
	})

	t.Run("U6 no-DD single class keeps Init compatibility and acquire guards dev zero without leaking RLock", func(t *testing.T) {
		t.Run("keyboard only", func(t *testing.T) {
			m := &mockLifecycle{
				slots: map[interception.Device]mockSlot{
					1: {hwids: []string{`ACPI\LEN0071`}},
				},
			}
			setupMockHID(t, m)

			if err := Init(); err != nil {
				t.Fatalf("Init() = %v", err)
			}
			assertHIDState(t, true, 0, 1)

			_, dev, unlock, err := acquireKeyboard()
			if err != nil || dev != 1 {
				t.Fatalf("acquireKeyboard() = (dev=%d, err=%v), want (1, nil)", dev, err)
			}
			unlock()

			_, dev, unlock, err = acquireMouse()
			if !errors.Is(err, ErrDevicesUnavailable) || dev != 0 || unlock != nil {
				t.Fatalf("acquireMouse() = (dev=%d, unlockNil=%v, err=%v), want (0, true, ErrDevicesUnavailable)", dev, unlock == nil, err)
			}
			if !initMutex.TryLock() {
				t.Fatal("initMutex remained read-locked after acquireMouse returned ErrDevicesUnavailable")
			}
			initMutex.Unlock()
		})

		t.Run("mouse only", func(t *testing.T) {
			m := &mockLifecycle{
				slots: map[interception.Device]mockSlot{
					11: {hwids: []string{`HID\VID_046D&PID_C52F&MI_00`}},
				},
			}
			setupMockHID(t, m)

			if err := Init(); err != nil {
				t.Fatalf("Init() = %v", err)
			}
			assertHIDState(t, true, 11, 0)

			_, dev, unlock, err := acquireKeyboard()
			if !errors.Is(err, ErrDevicesUnavailable) || dev != 0 || unlock != nil {
				t.Fatalf("acquireKeyboard() = (dev=%d, unlockNil=%v, err=%v), want (0, true, ErrDevicesUnavailable)", dev, unlock == nil, err)
			}
			if !initMutex.TryLock() {
				t.Fatal("initMutex remained read-locked after acquireKeyboard returned ErrDevicesUnavailable")
			}
			initMutex.Unlock()
		})
	})

	t.Run("U8 missing export and HWID query anomalies fail safely", func(t *testing.T) {
		t.Run("missing export propagates ErrSymbolsMissing", func(t *testing.T) {
			m := &mockLifecycle{loadErr: ErrSymbolsMissing}
			setupMockHID(t, m)

			if err := Init(); !errors.Is(err, ErrSymbolsMissing) {
				t.Fatalf("Init() = %v, want ErrSymbolsMissing", err)
			}
			assertHIDState(t, false, 0, 0)
		})

		t.Run("query anomaly with no valid physical device fails closed", func(t *testing.T) {
			wantErr := errors.New("corrupt hwid")
			m := &mockLifecycle{
				slots: map[interception.Device]mockSlot{
					1:  {hwids: []string{`ACPI\LEN0071`}},
					11: {err: wantErr},
					12: {hwids: []string{`HID\ddhid63340&Col03`}},
				},
			}
			setupMockHID(t, m)

			err := Init()
			if !errors.Is(err, wantErr) {
				t.Fatalf("Init() = %v, want wrapped %v", err, wantErr)
			}
			if len(m.destroyCalls) != 1 || m.unloadCalls != 1 {
				t.Fatalf("cleanup = (destroy=%d, unload=%d), want (1, 1)", len(m.destroyCalls), m.unloadCalls)
			}
			assertHIDState(t, false, 0, 0)
		})

		t.Run("query anomaly skipped when valid physical device follows", func(t *testing.T) {
			m := &mockLifecycle{
				slots: map[interception.Device]mockSlot{
					1:  {err: errors.New("odd byte count")},
					2:  {hwids: []string{`HID\VID_046D&PID_C31C&MI_00`}},
					11: {hwids: []string{`HID\VID_046D&PID_C52F&MI_00`}},
				},
			}
			setupMockHID(t, m)

			if err := Init(); err != nil {
				t.Fatalf("Init() = %v", err)
			}
			assertHIDState(t, true, 11, 2)
		})
	})
}
