package winput

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/rpdg/winput/hid"
)

var errStopBeforeInput = errors.New("stop before real input")

func TestNewInputHIDDirect(t *testing.T) {
	prev := getBackend()
	defer setGlobalBackend(prev)

	cases := []struct {
		name    string
		initErr error
		wantErr error
	}{
		{"success", nil, nil},
		{"driver not installed", hid.ErrDriverNotInstalled, ErrDriverNotInstalled},
		{"devices unavailable", hid.ErrDevicesUnavailable, ErrDLLLoadFailed},
		{"symbols missing", hid.ErrSymbolsMissing, ErrDLLLoadFailed},
		{"library missing", hid.ErrLibraryNotFound, ErrDLLLoadFailed},
		{"other initialization failure", errors.New("other init failure"), ErrDLLLoadFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setGlobalBackend(BackendMessage)
			calls := 0
			restoreInit := swapInit(func() error {
				calls++
				return tc.initErr
			})
			defer restoreInit()

			in, err := NewInput(BackendHID)
			if calls != 1 {
				t.Fatalf("NewInput(HID) init calls = %d, want 1", calls)
			}
			if tc.wantErr == nil {
				if err != nil || in == nil || !in.valid || in.backend != BackendHID {
					t.Fatalf("NewInput(HID) = %+v, %v", in, err)
				}
			} else {
				if in != nil {
					t.Fatalf("failed NewInput returned %+v", in)
				}
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("NewInput error = %v, want %v", err, tc.wantErr)
				}
				if tc.wantErr == ErrDriverNotInstalled && errors.Is(err, ErrDLLLoadFailed) {
					t.Fatalf("driver error also matched ErrDLLLoadFailed: %v", err)
				}
				if tc.wantErr == ErrDLLLoadFailed && !errors.Is(err, ErrDLLLoadFailed) {
					t.Fatalf("initialization error did not match ErrDLLLoadFailed: %v", err)
				}
			}
			if got := getBackend(); got != BackendMessage {
				t.Fatalf("NewInput(HID) changed global backend to %v", got)
			}
		})
	}

	t.Run("second HID construction keeps the first executor", func(t *testing.T) {
		setGlobalBackend(BackendMessage)
		calls := 0
		restoreInit := swapInit(func() error {
			calls++
			return nil
		})
		defer restoreInit()
		first, err := NewInput(BackendHID)
		if err != nil || first == nil {
			t.Fatalf("first NewInput = %v, %v", first, err)
		}
		second, err := NewInput(BackendHID)
		if err != nil || second == nil {
			t.Fatalf("second NewInput = %v, %v", second, err)
		}
		if calls != 2 {
			t.Fatalf("init calls = %d, want 2", calls)
		}
		if !first.valid || first.backend != BackendHID || !second.valid || second.backend != BackendHID {
			t.Fatalf("executors = %+v / %+v", first, second)
		}
		if got := getBackend(); got != BackendMessage {
			t.Fatalf("construction changed global backend to %v", got)
		}
	})

	t.Run("invalid backend never initializes", func(t *testing.T) {
		setGlobalBackend(BackendMessage)
		calls := 0
		restoreInit := swapInit(func() error {
			calls++
			return nil
		})
		defer restoreInit()
		for _, bad := range []Backend{Backend(-1), Backend(2), Backend(99)} {
			in, err := NewInput(bad)
			if in != nil || !errors.Is(err, ErrInvalidBackend) {
				t.Fatalf("NewInput(%d) = %v, %v", bad, in, err)
			}
		}
		if calls != 0 {
			t.Fatalf("invalid backends initialized HID %d times", calls)
		}
		if got := getBackend(); got != BackendMessage {
			t.Fatalf("invalid NewInput changed global backend to %v", got)
		}
	})
}

func TestAcquireBackendFixedSkipsGlobal(t *testing.T) {
	prev := getBackend()
	defer setGlobalBackend(prev)
	setGlobalBackend(BackendHID)
	calls := 0
	restoreInit := swapInit(func() error {
		calls++
		return errors.New("fixed path must not initialize")
	})
	defer restoreInit()

	fixed := BackendMessage
	cb, err := acquireBackend(&fixed)
	if err != nil || cb != fixed {
		t.Fatalf("acquireBackend(fixed) = %v, %v", cb, err)
	}
	if calls != 0 {
		t.Fatalf("fixed backend initialized HID %d times", calls)
	}

	if _, err := acquireBackend(nil); !errors.Is(err, ErrDLLLoadFailed) {
		t.Fatalf("global HID resolution = %v, want ErrDLLLoadFailed", err)
	}
	if calls != 1 {
		t.Fatalf("global HID resolution init calls = %d, want 1", calls)
	}
}

func TestInputFacadesUseFixedBackend(t *testing.T) {
	hwnd := createStaticWindow(true)
	if hwnd == 0 {
		t.Fatal("CreateWindowEx failed")
	}
	defer destroyWindow(hwnd)
	w := &Window{HWND: hwnd}

	prev := getBackend()
	defer setGlobalBackend(prev)

	for _, fixed := range []Backend{BackendMessage, BackendHID} {
		t.Run(backendName(fixed), func(t *testing.T) {
			global := BackendMessage
			if fixed == BackendMessage {
				global = BackendHID
			}
			setGlobalBackend(global)
			inits := 0
			restoreInit := swapInit(func() error {
				inits++
				return errors.New("facade must not initialize HID")
			})
			defer restoreInit()

			in := &Input{backend: fixed, valid: true}
			win, err := in.ForWindow(w)
			if err != nil {
				t.Fatal(err)
			}
			desk, err := in.Desktop()
			if err != nil {
				t.Fatal(err)
			}

			var seen []Backend
			restoreBackend := swapAcquireBackend(func(got *Backend) (Backend, error) {
				if got == nil {
					t.Error("facade passed nil fixed backend")
					return 0, errStopBeforeInput
				}
				seen = append(seen, *got)
				return 0, errStopBeforeInput
			})
			defer restoreBackend()

			checks := []struct {
				name string
				call func() error
			}{
				{"Window.Move", func() error { return win.Move(0, 0) }},
				{"Window.MoveRel", func() error { return win.MoveRel(1, 1) }},
				{"Window.Click", func() error { return win.Click(0, 0) }},
				{"Window.ClickRight", func() error { return win.ClickRight(0, 0) }},
				{"Window.ClickMiddle", func() error { return win.ClickMiddle(0, 0) }},
				{"Window.DoubleClick", func() error { return win.DoubleClick(0, 0) }},
				{"Window.Scroll", func() error { return win.Scroll(0, 0, 120) }},
				{"Window.KeyDown", func() error { return win.KeyDown(KeyA) }},
				{"Window.KeyUp", func() error { return win.KeyUp(KeyA) }},
				{"Window.Press", func() error { return win.Press(KeyA) }},
				{"Window.PressHotkey", func() error { return win.PressHotkey(KeyA) }},
				{"Window.Type", func() error { return win.Type("a") }},
				{"Desktop.MoveMouseTo", func() error { return desk.MoveMouseTo(0, 0) }},
				{"Desktop.ClickMouseAt", func() error { return desk.ClickMouseAt(0, 0) }},
				{"Desktop.DoubleClickMouseAt", func() error { return desk.DoubleClickMouseAt(0, 0) }},
				{"Desktop.ClickRightMouseAt", func() error { return desk.ClickRightMouseAt(0, 0) }},
				{"Desktop.ClickMiddleMouseAt", func() error { return desk.ClickMiddleMouseAt(0, 0) }},
				{"Desktop.MouseDown", func() error { return desk.MouseDown(MouseLeft) }},
				{"Desktop.MouseUp", func() error { return desk.MouseUp(MouseLeft) }},
				{"Desktop.ClickMouse", func() error { return desk.ClickMouse() }},
				{"Desktop.MoveMouseBy", func() error { return desk.MoveMouseBy(1, 1) }},
				{"Desktop.MoveMouseToExact", func() error { return desk.MoveMouseToExact(0, 0) }},
				{"Desktop.ScrollWheel", func() error { return desk.ScrollWheel(1) }},
				{"Desktop.PressFor", func() error { return desk.PressFor(KeyA, 10*time.Millisecond) }},
				{"Desktop.KeyDown", func() error { return desk.KeyDown(KeyA) }},
				{"Desktop.KeyUp", func() error { return desk.KeyUp(KeyA) }},
				{"Desktop.Press", func() error { return desk.Press(KeyA) }},
				{"Desktop.PressHotkey", func() error { return desk.PressHotkey(KeyA) }},
				{"Desktop.Type", func() error { return desk.Type("a") }},
				{"Desktop.PasteByClipboard", func() error { return desk.PasteByClipboard("a") }},
			}
			for _, check := range checks {
				if err := check.call(); !errors.Is(err, errStopBeforeInput) {
					t.Errorf("%s error = %v, want the fixed-backend seam error", check.name, err)
				}
			}
			if len(seen) != 30 {
				t.Fatalf("facade backend resolutions = %d, want 30", len(seen))
			}
			for i, got := range seen {
				if got != fixed {
					t.Fatalf("resolution %d used backend %v, want %v", i, got, fixed)
				}
			}
			if inits != 0 {
				t.Fatalf("facades initialized HID %d times", inits)
			}
		})
	}
}

func TestInputFacadeMethodSet(t *testing.T) {
	windowMethods := methodNames(reflect.TypeOf(&WindowInput{}))
	desktopMethods := methodNames(reflect.TypeOf(&DesktopInput{}))
	wantWindow := []string{
		"Click", "ClickMiddle", "ClickRight", "DoubleClick", "KeyDown", "KeyUp",
		"Move", "MoveRel", "Press", "PressHotkey", "Scroll", "Type",
	}
	wantDesktop := []string{
		"ClickMiddleMouseAt", "ClickMouse", "ClickMouseAt", "ClickRightMouseAt",
		"DoubleClickMouseAt", "KeyDown", "KeyUp", "MouseDown", "MouseUp",
		"MoveMouseBy", "MoveMouseTo", "MoveMouseToExact", "PasteByClipboard",
		"Press", "PressFor", "PressHotkey", "ScrollWheel", "Type",
	}
	if !reflect.DeepEqual(windowMethods, stringSet(wantWindow)) {
		t.Fatalf("WindowInput methods = %v, want %v", windowMethods, wantWindow)
	}
	if !reflect.DeepEqual(desktopMethods, stringSet(wantDesktop)) {
		t.Fatalf("DesktopInput methods = %v, want %v", desktopMethods, wantDesktop)
	}
	if _, ok := desktopMethods["PasteByClipboard"]; !ok {
		t.Fatal("DesktopInput lacks PasteByClipboard")
	}
	if _, ok := windowMethods["PasteByClipboard"]; ok {
		t.Fatal("WindowInput has PasteByClipboard")
	}
	for _, name := range []string{"GetCursorPos", "FindByTitle", "FindByClass", "FindByPID", "FindByProcessName", "Text", "Value", "DPI", "ClientRect", "ClientInfo", "IsValid", "IsVisible"} {
		if _, ok := desktopMethods[name]; ok {
			t.Fatalf("DesktopInput has query method %s", name)
		}
		if _, ok := windowMethods[name]; ok {
			t.Fatalf("WindowInput has query method %s", name)
		}
	}
}

func methodNames(t reflect.Type) map[string]struct{} {
	out := make(map[string]struct{}, t.NumMethod())
	for i := 0; i < t.NumMethod(); i++ {
		out[t.Method(i).Name] = struct{}{}
	}
	return out
}

func stringSet(names []string) map[string]struct{} {
	out := make(map[string]struct{}, len(names))
	for _, name := range names {
		out[name] = struct{}{}
	}
	return out
}

func backendName(b Backend) string {
	if b == BackendMessage {
		return "fixed_Message"
	}
	return "fixed_HID"
}

func swapAcquireBackend(fn func(*Backend) (Backend, error)) func() {
	prev := acquireBackend
	acquireBackend = fn
	return func() { acquireBackend = prev }
}
