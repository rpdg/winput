package winput

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"unsafe"

	"github.com/rpdg/winput/hid"
)

func TestDeriveWarnings(t *testing.T) {
	cases := []struct {
		rel    IntegrityRelation
		access UIAccess
		want   []ProbeWarning
	}{
		{IntegrityTargetHigher, UIAccessEnabled, []ProbeWarning{WarnUIPILikelyBlocked}},
		{IntegrityTargetHigher, UIAccessDisabled, []ProbeWarning{WarnUIPILikelyBlocked}},
		{IntegrityTargetHigher, UIAccessUnknown, []ProbeWarning{WarnUIPILikelyBlocked, WarnUIAccessUnknown}},
		{IntegrityTargetHigher, "", []ProbeWarning{WarnUIPILikelyBlocked, WarnUIAccessUnknown}},
		{IntegrityEqual, UIAccessEnabled, nil},
		{IntegrityEqual, UIAccessUnknown, []ProbeWarning{WarnUIAccessUnknown}},
		{IntegrityTargetLower, UIAccessDisabled, nil},
		{IntegrityUnknown, UIAccessEnabled, []ProbeWarning{WarnIntegrityUnknown}},
		{IntegrityUnknown, UIAccessUnknown, []ProbeWarning{WarnIntegrityUnknown, WarnUIAccessUnknown}},
		{"", UIAccessEnabled, []ProbeWarning{WarnIntegrityUnknown}},
	}
	for _, tc := range cases {
		got := deriveWarnings(tc.rel, tc.access)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("deriveWarnings(%q, %q) = %#v, want %#v", tc.rel, tc.access, got, tc.want)
		}
	}
}

func TestClassifyHIDUsesSentinelsNotText(t *testing.T) {
	lib := fmt.Errorf("load: %w", hid.ErrLibraryNotFound)
	cases := []struct {
		err  error
		want HIDStatus
	}{
		{nil, HIDInitialized},
		{hid.ErrDriverNotInstalled, HIDDriverUnavailable},
		{fmt.Errorf("ctx: %w", hid.ErrDriverNotInstalled), HIDDriverUnavailable},
		{hid.ErrDevicesUnavailable, HIDDevicesUnavailable},
		{hid.ErrLibraryNotFound, HIDDLLUnavailable},
		{hid.ErrSymbolsMissing, HIDDLLUnavailable},
		{lib, HIDDLLUnavailable},
		{fmt.Errorf("no interception devices found"), HIDUnknown},
		{fmt.Errorf("library loaded but symbols missing"), HIDUnknown},
		{errors.New("something else"), HIDUnknown},
	}
	for _, tc := range cases {
		got := classifyHID(tc.err)
		if got != tc.want {
			t.Errorf("classifyHID(%v) = %q, want %q", tc.err, got, tc.want)
		}
		if got == "" {
			t.Errorf("classifyHID(%v) returned an empty status", tc.err)
		}
	}
	if HIDStatus("") == HIDInitialized {
		t.Fatal("zero HIDStatus must not mean initialized")
	}
	if IntegrityRelation("") == IntegrityEqual || UIAccess("") == UIAccessEnabled {
		t.Fatal("zero fact values must not mean a confirmed fact")
	}
}

func TestTranslateHIDInitKeepsPublicErrors(t *testing.T) {
	driver := translateHIDInit(hid.ErrDriverNotInstalled)
	if !errors.Is(driver, ErrDriverNotInstalled) || errors.Is(driver, ErrDLLLoadFailed) {
		t.Fatalf("driver error = %v", driver)
	}
	for _, err := range []error{hid.ErrDevicesUnavailable, hid.ErrSymbolsMissing, hid.ErrLibraryNotFound, errors.New("other")} {
		got := translateHIDInit(err)
		if !errors.Is(got, ErrDLLLoadFailed) {
			t.Fatalf("translate(%v) = %v, want ErrDLLLoadFailed", err, got)
		}
		if errors.Is(got, ErrDriverNotInstalled) || errors.Is(got, hid.ErrDevicesUnavailable) || errors.Is(got, hid.ErrSymbolsMissing) {
			t.Fatalf("translate(%v) exposed a hid sentinel: %v", err, got)
		}
	}
}

func TestResolveIdentity(t *testing.T) {
	base := idSnap{valid: true, idOK: true, pid: 7, tid: 9, class: classSnap{known: true, value: "Edit"}}
	cases := []struct {
		name  string
		start idSnap
		end   idSnap
		class string
		err   error
	}{
		{"invalid start", idSnap{}, base, "", ErrWindowGone},
		{"invalid end", base, idSnap{idOK: true}, "", ErrWindowGone},
		{"start pid missing", idSnap{valid: true}, base, "", ErrWindowGone},
		{"end tid missing", base, idSnap{valid: true, pid: 7}, "", ErrWindowGone},
		{"pid changed", base, idSnap{valid: true, idOK: true, pid: 8, tid: 9, class: base.class}, "", ErrTargetChanged},
		{"tid changed", base, idSnap{valid: true, idOK: true, pid: 7, tid: 10, class: base.class}, "", ErrTargetChanged},
		{"class changed", base, idSnap{valid: true, idOK: true, pid: 7, tid: 9, class: classSnap{known: true, value: "Button"}}, "", ErrTargetChanged},
		{"end class unread keeps start", base, idSnap{valid: true, idOK: true, pid: 7, tid: 9}, "Edit", nil},
		{"start unread uses end", idSnap{valid: true, idOK: true, pid: 7, tid: 9}, base, "Edit", nil},
		{"both unread", idSnap{valid: true, idOK: true, pid: 7, tid: 9}, idSnap{valid: true, idOK: true, pid: 7, tid: 9}, "", nil},
		{"empty class is a value", idSnap{valid: true, idOK: true, pid: 1, tid: 2, class: classSnap{known: true, value: ""}}, idSnap{valid: true, idOK: true, pid: 1, tid: 2, class: classSnap{known: true, value: ""}}, "", nil},
		{"empty then different", idSnap{valid: true, idOK: true, pid: 1, tid: 2, class: classSnap{known: true, value: ""}}, idSnap{valid: true, idOK: true, pid: 1, tid: 2, class: classSnap{known: true, value: "Edit"}}, "", ErrTargetChanged},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveIdentity(tc.start, tc.end)
			if !errors.Is(err, tc.err) || got != tc.class {
				t.Fatalf("resolve = %q, %v; want class %q err %v", got, err, tc.class, tc.err)
			}
		})
	}
}

func TestProbeTargetNilAndSeams(t *testing.T) {
	inits := 0
	restoreInit := swapInit(func() error {
		inits++
		return errors.New("probe must not init")
	})
	defer restoreInit()

	report, err := ProbeTarget(nil)
	if report != nil || !errors.Is(err, ErrWindowGone) {
		t.Fatalf("nil window = %v, %v", report, err)
	}
	report, err = ProbeTarget(&Window{})
	if report != nil || !errors.Is(err, ErrWindowGone) {
		t.Fatalf("zero hwnd = %v, %v", report, err)
	}

	reads := 0
	restoreID := swapIdentity(func(uintptr) idSnap {
		reads++
		class := "Edit"
		if reads > 1 {
			class = "Button"
		}
		return idSnap{valid: true, idOK: true, pid: 4, tid: 5, class: classSnap{known: true, value: class}}
	})
	_, err = ProbeTarget(&Window{HWND: 1})
	restoreID()
	if !errors.Is(err, ErrTargetChanged) {
		t.Fatalf("changed class = %v", err)
	}

	n := 0
	restoreID = swapIdentity(func(uintptr) idSnap {
		n++
		return idSnap{valid: true, idOK: true, pid: 4, tid: 5}
	})
	restoreFacts := swapFacts(func(uint32) (string, IntegrityRelation, UIAccess) {
		return "", IntegrityUnknown, UIAccessUnknown
	})
	report, err = ProbeTarget(&Window{HWND: 1})
	restoreID()
	restoreFacts()
	if err != nil || report == nil {
		t.Fatalf("unknown facts = %v, %v", report, err)
	}
	if report.Features.TargetClass != "" || report.Features.ProcessName != "" {
		t.Fatalf("facts = %+v", report.Features)
	}
	if report.Features.IntegrityRelation != IntegrityUnknown || report.Features.CallerUIAccess != UIAccessUnknown {
		t.Fatalf("tokens = %+v", report.Features)
	}
	want := []ProbeWarning{WarnIntegrityUnknown, WarnUIAccessUnknown}
	if !reflect.DeepEqual(report.Warnings, want) {
		t.Fatalf("warnings = %#v", report.Warnings)
	}
	if inits != 0 {
		t.Fatalf("ProbeTarget called HID init %d times", inits)
	}
	assertNoSuggestionFields(t)
}

func TestProbeTargetRechecksIdentityAfterFacts(t *testing.T) {
	w := &Window{HWND: 1}
	changed := false
	restoreID := swapIdentity(func(hwnd uintptr) idSnap {
		if hwnd != 1 {
			t.Fatalf("identity read used hwnd %d", hwnd)
		}
		class := "Edit"
		pid := uint32(9)
		if changed {
			class = "Button"
			pid = 10
		}
		return idSnap{valid: true, idOK: true, pid: pid, tid: 3, class: classSnap{known: true, value: class}}
	})
	defer restoreID()
	facts := 0
	restoreFacts := swapFacts(func(pid uint32) (string, IntegrityRelation, UIAccess) {
		facts++
		if pid != 9 {
			t.Fatalf("facts collected for pid %d", pid)
		}
		changed = true
		w.HWND = 99
		return "notepad.exe", IntegrityEqual, UIAccessEnabled
	})
	defer restoreFacts()

	report, err := ProbeTarget(w)
	if facts != 1 {
		t.Fatalf("collectFacts calls = %d", facts)
	}
	if report != nil || !errors.Is(err, ErrTargetChanged) {
		t.Fatalf("change during facts = report %v, err %v", report, err)
	}

	changed = false
	w.HWND = 1
	restoreFacts()
	restoreID()
	restoreFacts = swapFacts(func(uint32) (string, IntegrityRelation, UIAccess) {
		changed = true
		return "notepad.exe", IntegrityEqual, UIAccessEnabled
	})
	restoreID = swapIdentity(func(uintptr) idSnap {
		if changed {
			return idSnap{}
		}
		return idSnap{valid: true, idOK: true, pid: 9, tid: 3, class: classSnap{known: true, value: "Edit"}}
	})
	report, err = ProbeTarget(w)
	restoreFacts()
	restoreID()
	if report != nil || !errors.Is(err, ErrWindowGone) {
		t.Fatalf("gone during facts = report %v, err %v", report, err)
	}
}

func TestNewInputAndTaskFacade(t *testing.T) {
	prev := getBackend()
	defer func() { _ = SetBackend(prev) }()

	inits := 0
	restore := swapInit(func() error {
		inits++
		return errors.New("unexpected init")
	})
	defer restore()

	msg, err := NewInput(BackendMessage)
	if err != nil || msg == nil || !msg.valid || msg.backend != BackendMessage {
		t.Fatalf("message input = %+v, %v", msg, err)
	}
	if inits != 0 {
		t.Fatal("BackendMessage initialized HID")
	}
	for _, bad := range []Backend{2, Backend(-1), Backend(99)} {
		got, err := NewInput(bad)
		if got != nil || !errors.Is(err, ErrInvalidBackend) {
			t.Fatalf("NewInput(%d) = %v, %v", bad, got, err)
		}
	}
	if inits != 0 {
		t.Fatal("illegal backend initialized HID")
	}
	if ErrInvalidBackend.Error() != "invalid input backend" {
		t.Fatalf("error text = %q", ErrInvalidBackend.Error())
	}

	var zero Input
	if facade, err := zero.ForWindow(&Window{}); facade != nil || !errors.Is(err, ErrInputInvalid) {
		t.Fatalf("zero ForWindow = %v, %v", facade, err)
	}
	if desk, err := zero.Desktop(); desk != nil || !errors.Is(err, ErrInputInvalid) {
		t.Fatalf("zero Desktop = %v, %v", desk, err)
	}
	var nilInput *Input
	if _, err := nilInput.ForWindow(nil); !errors.Is(err, ErrInputInvalid) {
		t.Fatalf("nil ForWindow = %v", err)
	}
	var win WindowInput
	if err := win.Click(0, 0); !errors.Is(err, ErrInputInvalid) {
		t.Fatalf("zero window facade = %v", err)
	}
	var desk DesktopInput
	if err := desk.MoveMouseTo(0, 0); !errors.Is(err, ErrInputInvalid) {
		t.Fatalf("zero desktop facade = %v", err)
	}
	var nilWin *WindowInput
	if err := nilWin.Scroll(0, 0, 120); !errors.Is(err, ErrInputInvalid) {
		t.Fatalf("nil window facade = %v", err)
	}
	var nilDesk *DesktopInput
	if err := nilDesk.KeyDown(KeyA); !errors.Is(err, ErrInputInvalid) {
		t.Fatalf("nil desktop facade = %v", err)
	}

	facade, err := msg.ForWindow(nil)
	if err != nil || facade == nil {
		t.Fatalf("ForWindow(nil) = %v, %v", facade, err)
	}
	if err := facade.Click(1, 1); !errors.Is(err, ErrWindowGone) {
		t.Fatalf("nil window click = %v", err)
	}

	if err := SetBackend(BackendHID); err == nil {
		t.Fatal("SetBackend(HID) succeeded while init is stubbed")
	}
	inits = 0
	_ = SetBackend(Backend(42))
	if getBackend() != 42 {
		t.Fatalf("backend = %v", getBackend())
	}
	// The message executor must ignore the global backend, including HID.
	backendMutex.Lock()
	currentBackend = BackendHID
	backendMutex.Unlock()
	isolated, err := msg.Desktop()
	if err != nil {
		t.Fatal(err)
	}
	err = isolated.ScrollWheel(0)
	if !errors.Is(err, ErrInvalidScrollSteps) {
		t.Fatalf("isolated scroll = %v", err)
	}
	if inits != 0 {
		t.Fatal("task input read the global HID backend and initialized")
	}
	if msg.backend != BackendMessage {
		t.Fatal("SetBackend changed an existing executor")
	}
}

func TestNonHIDSetBackendStaysOnMessagePath(t *testing.T) {
	prev := getBackend()
	defer func() { _ = SetBackend(prev) }()
	if err := SetBackend(Backend(42)); err != nil {
		t.Fatal(err)
	}
	if getBackend() != 42 {
		t.Fatalf("stored %v", getBackend())
	}
	inits := 0
	restore := swapInit(func() error {
		inits++
		return errors.New("non-HID must not init")
	})
	defer restore()
	err := ScrollWheel(0)
	if !errors.Is(err, ErrInvalidScrollSteps) {
		t.Fatalf("ScrollWheel = %v", err)
	}
	if inits != 0 {
		t.Fatal("non-HID backend initialized HID")
	}
	status := ProbeHID()
	if status != HIDUnknown {
		t.Fatalf("ProbeHID under stub = %q", status)
	}
	if getBackend() != 42 {
		t.Fatal("ProbeHID changed the global backend")
	}
}

func TestWindowInputDirectedScroll(t *testing.T) {
	hwnd := createStaticWindow(true)
	if hwnd == 0 {
		t.Fatal("CreateWindowEx failed")
	}
	defer destroyWindow(hwnd)
	w := &Window{HWND: hwnd}
	if err := w.checkReady(); err != nil {
		t.Fatalf("visible static not ready: %v", err)
	}
	executor := &Input{backend: BackendHID, valid: true}
	facade, err := executor.ForWindow(w)
	if err != nil {
		t.Fatal(err)
	}
	err = facade.Scroll(1, 1, 120)
	if !errors.Is(err, ErrDirectedScrollUnsupported) {
		t.Fatalf("HID scroll = %v", err)
	}

	hidden := createStaticWindow(false)
	if hidden == 0 {
		t.Fatal("hidden CreateWindowEx failed")
	}
	defer destroyWindow(hidden)
	hw := &Window{HWND: hidden}
	if hw.IsValid() && hw.IsVisible() {
		t.Fatal("hidden window looks ready")
	}
	if err := hw.Move(0, 0); !errors.Is(err, ErrWindowNotVisible) {
		t.Fatalf("hidden Move = %v, want not visible", err)
	}
	report, err := ProbeTarget(hw)
	if err != nil || report == nil {
		t.Fatalf("hidden probe = %v, %v", report, err)
	}
	if report.Features.TargetClass != "Static" {
		t.Fatalf("class = %q", report.Features.TargetClass)
	}
	if strings.ContainsAny(report.Features.ProcessName, `\/:`) {
		t.Fatalf("process name is a path: %q", report.Features.ProcessName)
	}
	if report.Features.IntegrityRelation == "" || report.Features.CallerUIAccess == "" {
		t.Fatalf("empty tokens %+v", report.Features)
	}
	if !reflect.DeepEqual(report.Warnings, deriveWarnings(report.Features.IntegrityRelation, report.Features.CallerUIAccess)) {
		t.Fatalf("warnings = %#v", report.Warnings)
	}
	if _, ok := reflect.TypeOf(*report).FieldByName("SuggestedBackend"); ok {
		t.Fatal("probe has a suggestion field")
	}
}

func TestDesktopFacadeHasNoPasteOrQueries(t *testing.T) {
	methods := reflect.TypeOf(&DesktopInput{})
	for _, name := range []string{"PasteByClipboard", "GetCursorPos", "FindByTitle"} {
		if _, ok := methods.MethodByName(name); ok {
			t.Fatalf("DesktopInput has %s", name)
		}
	}
	if _, ok := reflect.TypeOf(&WindowInput{}).MethodByName("Text"); ok {
		t.Fatal("WindowInput has Text")
	}
}

func assertNoSuggestionFields(t *testing.T) {
	t.Helper()
	rt := reflect.TypeOf(TargetProbe{})
	for _, name := range []string{"SuggestedBackend", "HWND", "Confidence", "HIDStatus"} {
		if _, ok := rt.FieldByName(name); ok {
			t.Fatalf("TargetProbe has %s", name)
		}
	}
}

func swapInit(fn func() error) func() {
	prev := callHIDInit
	callHIDInit = fn
	return func() { callHIDInit = prev }
}

func swapIdentity(fn func(uintptr) idSnap) func() {
	prev := readIdentity
	readIdentity = fn
	return func() { readIdentity = prev }
}

func swapFacts(fn func(uint32) (string, IntegrityRelation, UIAccess)) func() {
	prev := collectFacts
	collectFacts = fn
	return func() { collectFacts = prev }
}

func createStaticWindow(visible bool) uintptr {
	class, err := syscall.UTF16PtrFromString("Static")
	if err != nil {
		return 0
	}
	var style uintptr
	off := int32(-32000)
	x := uintptr(off)
	y := uintptr(off)
	if visible {
		style = 0x80000000 | 0x10000000 // WS_POPUP | WS_VISIBLE
	}
	proc := syscall.NewLazyDLL("user32.dll").NewProc("CreateWindowExW")
	hwnd, _, _ := proc.Call(0, uintptr(unsafe.Pointer(class)), 0, style, x, y, 10, 10, 0, 0, 0, 0)
	return hwnd
}

func destroyWindow(hwnd uintptr) {
	proc := syscall.NewLazyDLL("user32.dll").NewProc("DestroyWindow")
	_, _, _ = proc.Call(hwnd)
}
