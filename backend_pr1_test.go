package winput

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/rpdg/winput/screen"
)

// recordingBackend deliberately implements only the nine input operations.
// It has no lifecycle, capability or cross-call state contract.
type recordingBackend struct {
	events []string
	fail   func(string) error
}

func (f *recordingBackend) record(event string) error {
	f.events = append(f.events, event)
	if f.fail != nil {
		return f.fail(event)
	}
	return nil
}
func (f *recordingBackend) KeyDown(k Key) error     { return f.record(fmt.Sprintf("down:%x", uint16(k))) }
func (f *recordingBackend) KeyUp(k Key) error       { return f.record(fmt.Sprintf("up:%x", uint16(k))) }
func (f *recordingBackend) MoveTo(x, y int32) error { return f.record(fmt.Sprintf("move:%d,%d", x, y)) }
func (f *recordingBackend) MoveToExact(x, y int32) error {
	return f.record(fmt.Sprintf("exact:%d,%d", x, y))
}
func (f *recordingBackend) MoveRelative(x, y int32) error {
	return f.record(fmt.Sprintf("relative:%d,%d", x, y))
}
func (f *recordingBackend) MouseButton(b MouseButton, down bool) error {
	return f.record(fmt.Sprintf("button:%d,%t", b, down))
}
func (f *recordingBackend) ClickAt(b MouseButton, x, y int32) error {
	return f.record(fmt.Sprintf("click:%d:%d,%d", b, x, y))
}
func (f *recordingBackend) DoubleClickAt(x, y int32) error {
	return f.record(fmt.Sprintf("double:%d,%d", x, y))
}
func (f *recordingBackend) Scroll(delta int32) error {
	return f.record(fmt.Sprintf("scroll:%d", delta))
}

func preservePR1Seams(t *testing.T) {
	t.Helper()
	backendMutex.RLock()
	prev := currentSelection
	backendMutex.RUnlock()
	init, acquire, down, up := callHIDInit, acquireBackend, keyDown, keyUp
	ancestor, foreground, setForeground := inputAncestorRoot, inputForegroundWindow, inputSetForeground
	vk, text := inputVirtualKey, inputWindowType
	cursor, client, bounds := inputCursorPos, inputClientToScreen, inputVirtualBounds
	typeSleep, hotkeySleep, scan := deviceTypeSleep, foregroundHotkeySleep, foregroundScanCode
	swap, paste := swapUnicodeText, pasteHotkey
	t.Cleanup(func() {
		backendMutex.Lock()
		currentSelection = prev
		backendMutex.Unlock()
		callHIDInit, acquireBackend, keyDown, keyUp = init, acquire, down, up
		inputAncestorRoot, inputForegroundWindow, inputSetForeground = ancestor, foreground, setForeground
		inputVirtualKey, inputWindowType = vk, text
		inputCursorPos, inputClientToScreen, inputVirtualBounds = cursor, client, bounds
		deviceTypeSleep, foregroundHotkeySleep, foregroundScanCode = typeSleep, hotkeySleep, scan
		swapUnicodeText, pasteHotkey = swap, paste
	})
}
func expectEvents(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %v; want %v", got, want)
	}
	t.Logf("sequence: %v", got)
}
func receivePR1(t *testing.T, ch <-chan error) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("transaction did not complete")
		return nil
	}
}

func TestPR1PrepareFailureSameKindAndUnknown(t *testing.T) {
	preservePR1Seams(t)
	original := &recordingBackend{}
	backendMutex.Lock()
	currentSelection = backendSelection{BackendHID, original}
	backendMutex.Unlock()
	calls := 0
	callHIDInit = func() error {
		calls++
		if !inputMutex.TryLock() {
			t.Fatal("prepare holds inputMutex")
		}
		inputMutex.Unlock()
		if !backendMutex.TryLock() {
			t.Fatal("prepare holds backendMutex")
		}
		backendMutex.Unlock()
		return errors.New("init failure")
	}
	if err := SetBackend(BackendHID); !errors.Is(err, ErrDLLLoadFailed) {
		t.Fatal(err)
	}
	if calls != 1 || currentSelection.device != original {
		t.Fatal("failed preparation changed selection or was skipped")
	}
	callHIDInit = func() error { calls++; return nil }
	if err := SetBackend(BackendHID); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || currentSelection.device != original {
		t.Fatal("same kind must prepare then retain original selection")
	}
	for _, kind := range []Backend{42, BackendMessage, 2, -1} {
		if err := SetBackend(kind); err != nil {
			t.Fatal(err)
		}
		if currentSelection.kind != kind || currentSelection.device != nil {
			t.Fatalf("unpaired selection: %+v", currentSelection)
		}
	}
	for _, kind := range []Backend{42, 2, -1} {
		if in, err := NewInput(kind); in != nil || !errors.Is(err, ErrInvalidBackend) {
			t.Fatalf("invalid constructor: %v %v", in, err)
		}
	}
	if calls != 2 {
		t.Fatalf("invalid kinds initialized HID: %d", calls)
	}
}

func TestPR1PrepareOutsideLocksAndCommitWaitsForPasteRestore(t *testing.T) {
	preservePR1Seams(t)
	setGlobalBackend(BackendMessage)
	entered, release := make(chan struct{}), make(chan struct{})
	prepared := make(chan struct{})
	var events []string
	swapUnicodeText = func(string) (func() error, error) {
		events = append(events, "swap")
		close(entered)
		<-release
		return func() error {
			requirePasteLock(t)
			if getBackend() != BackendMessage {
				t.Error("committed before restore")
			}
			events = append(events, "restore")
			return nil
		}, nil
	}
	pasteHotkey = func(sel backendSelection, _ ...Key) error {
		if sel.kind != BackendMessage || sel.device != nil {
			t.Errorf("changed selection: %+v", sel)
		}
		events = append(events, "keys")
		return nil
	}
	callHIDInit = func() error {
		if inputMutex.TryLock() {
			inputMutex.Unlock()
			t.Error("paste transaction did not retain input lock")
		}
		if !backendMutex.TryLock() {
			t.Error("preparation holds backend lock")
		} else {
			backendMutex.Unlock()
		}
		close(prepared)
		return nil
	}
	pasteDone := make(chan error, 1)
	go func() { pasteDone <- PasteByClipboard("text") }()
	<-entered
	setDone := make(chan error, 1)
	go func() { setDone <- SetBackend(BackendHID) }()
	<-prepared
	// The held transaction excludes commits; taking the backend read lock here
	// also checks that preparation did not introduce reverse-lock waiting.
	if getBackend() != BackendMessage {
		t.Error("selection changed inside transaction")
	}
	close(release)
	if err := receivePR1(t, pasteDone); err != nil {
		t.Fatal(err)
	}
	if err := receivePR1(t, setDone); err != nil {
		t.Fatal(err)
	}
	expectEvents(t, events, []string{"swap", "keys", "restore"})
	if getBackend() != BackendHID || currentSelection.device == nil {
		t.Fatal("commit did not publish complete selection")
	}
}

func TestPR1SlowPrepareAllowsOldInputAndFixedInput(t *testing.T) {
	preservePR1Seams(t)
	setGlobalBackend(BackendMessage)
	entered, release := make(chan struct{}), make(chan struct{})
	callHIDInit = func() error { close(entered); <-release; return nil }
	done := make(chan error, 1)
	go func() { done <- SetBackend(BackendHID) }()
	<-entered
	if !backendMutex.TryLock() {
		close(release)
		t.Fatal("slow preparation holds backendMutex")
	}
	backendMutex.Unlock()
	var kinds []Backend
	keyDown = func(sel backendSelection, _ uintptr, _ Key) error { kinds = append(kinds, sel.kind); return nil }
	keyUp = keyDown
	fixed := backendSelection{kind: 42}
	// These calls would deadlock if SetBackend held inputMutex during preparation.
	inputDone := make(chan error, 1)
	go func() {
		if err := Press(KeyA, 0); err != nil {
			inputDone <- err
			return
		}
		inputDone <- pressGlobal(KeyA, 0, &fixed)
	}()
	err := receivePR1(t, inputDone)
	close(release)
	if setErr := receivePR1(t, done); setErr != nil {
		t.Fatal(setErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(kinds, []Backend{BackendMessage, BackendMessage, 42, 42}) {
		t.Fatal(kinds)
	}
}

func TestPR1GlobalSnapshotNeverRereadsAfterCheck(t *testing.T) {
	preservePR1Seams(t)
	fake := &recordingBackend{}
	backendMutex.Lock()
	currentSelection = backendSelection{BackendHID, fake}
	backendMutex.Unlock()
	callHIDInit = func() error {
		// Model a changed global value at the old check/get seam. Never call
		// SetBackend under inputMutex; the test deliberately edits only its fixture.
		setGlobalBackend(42)
		return nil
	}
	for _, call := range []func() error{
		func() error { return Press(KeyA, 0) },
		func() error { return PressHotkey(KeyA) },
	} {
		backendMutex.Lock()
		currentSelection = backendSelection{BackendHID, fake}
		backendMutex.Unlock()
		fake.events = nil
		if err := call(); err != nil {
			t.Fatal(err)
		}
		expectEvents(t, fake.events, []string{"down:1e", "up:1e"})
	}

}

func TestPR1TypeDeviceSequenceAndTiming(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprint(explicit), func(t *testing.T) {
			preservePR1Seams(t)
			f := &recordingBackend{}
			sel := backendSelection{BackendHID, f}
			inputAncestorRoot = func(uintptr) uintptr { f.record("root"); return 9 }
			inputForegroundWindow = func() uintptr { f.record("foreground"); return 9 }
			deviceTypeSleep = func(d time.Duration) { f.record("sleep:" + d.String()) }
			if err := typeDevice(sel, "Aa", 7*time.Millisecond, explicit, 9); err != nil {
				t.Fatal(err)
			}
			want := []string{"root", "foreground", "down:2a", "sleep:10ms", "root", "foreground", "down:1e", "sleep:" + defaultPressHold.String(), "up:1e", "up:2a"}
			if explicit {
				want = append(want, "sleep:7ms")
			}
			want = append(want, "root", "foreground", "down:1e", "sleep:"+defaultPressHold.String(), "up:1e")
			expectEvents(t, f.events, want)
		})
	}
}
func TestPR1TypeDevicePreflightAndFailureCleanup(t *testing.T) {
	preservePR1Seams(t)
	f := &recordingBackend{}
	sel := backendSelection{BackendHID, f}
	deviceTypeSleep = func(time.Duration) {}
	if err := typeDevice(sel, "a中", 0, true, 0); !errors.Is(err, ErrUnsupportedKey) || len(f.events) != 0 {
		t.Fatalf("preflight: %v %v", err, f.events)
	}
	downErr, upErr := errors.New("down"), errors.New("up")
	f.fail = func(event string) error {
		if event == "down:1e" {
			return downErr
		}
		if event == "up:2a" {
			return upErr
		}
		return nil
	}
	if err := typeDevice(sel, "A", 0, false, 0); !errors.Is(err, downErr) || !errors.Is(err, upErr) {
		t.Fatal(err)
	}
	expectEvents(t, f.events, []string{"down:2a", "down:1e", "up:2a", "up:2a"})
	f.events = nil
	f.fail = nil
	checks := 0
	inputAncestorRoot = func(uintptr) uintptr { return 1 }
	inputForegroundWindow = func() uintptr {
		checks++
		if checks == 1 {
			return 1
		}
		return 2
	}
	if err := typeDevice(sel, "A", 0, false, 1); !errors.Is(err, ErrTargetNotForeground) {
		t.Fatal(err)
	}
	expectEvents(t, f.events, []string{"down:2a", "up:2a"})
	if checks != 2 {
		t.Fatalf("foreground checks=%d", checks)
	}
}

func TestPR1UnknownBackendThreeMessagePaths(t *testing.T) {
	preservePR1Seams(t)
	hwnd := createStaticWindow(true)
	if hwnd == 0 {
		t.Fatal("CreateWindowEx failed")
	}
	defer destroyWindow(hwnd)
	w := &Window{HWND: hwnd}
	if err := SetBackend(42); err != nil {
		t.Fatal(err)
	}
	callHIDInit = func() error { t.Error("unknown kind initialized HID"); return errors.New("unexpected init") }
	textCalls := 0
	inputWindowType = func(h uintptr, text string, gap time.Duration) error {
		textCalls++
		if h != hwnd || text != "中" || gap != 0 {
			t.Error("wrong Message Type arguments")
		}
		return nil
	}
	if err := w.Type("中", 0); err != nil || textCalls != 1 {
		t.Fatalf("Window.Type=%v calls=%d", err, textCalls)
	}
	inputVirtualKey = func(Key) (uintptr, bool) { return 0, false }
	if err := validateKey(currentSelection, hwnd, KeyA); !errors.Is(err, ErrUnsupportedKey) {
		t.Fatalf("VK check skipped: %v", err)
	}
	inputVirtualKey = func(Key) (uintptr, bool) { return 65, true }
	foreground := uintptr(99)
	inputAncestorRoot = func(uintptr) uintptr { return 7 }
	inputForegroundWindow = func() uintptr { return foreground }
	var events []string
	inputSetForeground = func(h uintptr) error {
		events = append(events, fmt.Sprintf("foreground:%d", h))
		foreground = h
		return nil
	}
	foregroundScanCode = func(k Key, up bool) error {
		events = append(events, fmt.Sprintf("scan:%x:%t", uint16(k), up))
		return nil
	}
	foregroundHotkeySleep = func(time.Duration) {}
	keyDown = func(backendSelection, uintptr, Key) error {
		t.Error("modifier hotkey bypassed foreground sender")
		return nil
	}
	if err := w.PressHotkey(KeyCtrl, KeyV); err != nil {
		t.Fatal(err)
	}
	expectEvents(t, events, []string{"foreground:7", "scan:1d:false", "scan:2f:false", "scan:2f:true", "scan:1d:true", "foreground:99"})
}

func TestPR1ForegroundHotkeyCleanupBeforeRestore(t *testing.T) {
	preservePR1Seams(t)
	inputVirtualKey = func(Key) (uintptr, bool) { return 65, true }
	inputAncestorRoot = func(uintptr) uintptr { return 7 }
	fg := uintptr(99)
	inputForegroundWindow = func() uintptr { return fg }
	downErr, upErr, restoreErr := errors.New("down"), errors.New("up"), errors.New("restore")
	var events []string
	inputSetForeground = func(h uintptr) error {
		events = append(events, fmt.Sprintf("foreground:%d", h))
		fg = h
		if h == 99 {
			return restoreErr
		}
		return nil
	}
	foregroundHotkeySleep = func(time.Duration) {}
	foregroundScanCode = func(k Key, up bool) error {
		events = append(events, fmt.Sprintf("scan:%x:%t", uint16(k), up))
		if k == KeyV {
			return downErr
		}
		if up {
			return upErr
		}
		return nil
	}
	err := pressHotkeyForeground(7, []Key{KeyCtrl, KeyV})
	if !errors.Is(err, downErr) || !errors.Is(err, upErr) || !errors.Is(err, ErrPermissionDenied) {
		t.Fatal(err)
	}
	// The original restore wrapper retains text, not errors.Is(restoreErr).
	expectEvents(t, events, []string{"foreground:7", "scan:1d:false", "scan:2f:false", "scan:1d:true", "scan:1d:true", "foreground:99"})
}

func TestPR1MouseDeviceMappings(t *testing.T) {
	preservePR1Seams(t)
	f := &recordingBackend{}
	sel := backendSelection{BackendHID, f}
	inputCursorPos = func() (int32, int32, error) { return 10, 20, nil }
	inputClientToScreen = func(_ uintptr, x, y int32) (int32, int32, error) { return x + 100, y + 200, nil }
	inputVirtualBounds = func() screen.Rect { return screen.Rect{Left: -100, Top: -100, Right: 500, Bottom: 500} }
	if err := moveImpl(sel, 1, 2, 3, true); err != nil {
		t.Fatal(err)
	}
	if err := moveImpl(sel, 1, 2, 3, false); err != nil {
		t.Fatal(err)
	}
	hwnd := createStaticWindow(true)
	if hwnd == 0 {
		t.Fatal("CreateWindowEx failed")
	}
	defer destroyWindow(hwnd)
	w := &Window{HWND: hwnd}
	checks := []func() error{
		func() error { return moveMouseTo(1, 2, &sel) },
		func() error { return moveMouseBy(3, 4, &sel) },
		func() error { return moveMouseToExact(5, 6, &sel) },
		func() error { return clickMouseAt(7, 8, &sel) },
		func() error { return clickRightMouseAt(7, 8, &sel) },
		func() error { return clickMiddleMouseAt(7, 8, &sel) },
		func() error { return doubleClickMouseAt(7, 8, &sel) },
		func() error { return clickWindow(w, 7, 8, &sel) },
		func() error { return clickRightWindow(w, 7, 8, &sel) },
		func() error { return clickMiddleWindow(w, 7, 8, &sel) },
		func() error { return doubleClickWindow(w, 7, 8, &sel) },
		func() error { return scrollWheel(-2, &sel) },
		func() error { return clickMouse(&sel) },
	}
	for _, check := range checks {
		if err := check(); err != nil {
			t.Fatal(err)
		}
	}
	expectEvents(t, f.events, []string{"move:12,23", "move:102,203", "move:1,2", "relative:3,4", "exact:5,6", "click:1:7,8", "click:2:7,8", "click:3:7,8", "double:7,8", "click:1:107,208", "click:2:107,208", "click:3:107,208", "double:107,208", "scroll:-240", "button:1,true", "button:1,false"})
	before := len(f.events)
	if err := moveMouseToExact(500, 0, &sel); !errors.Is(err, ErrOutsideVirtualDesktop) {
		t.Fatal(err)
	}
	if err := scrollWindow(w, 0, 0, 0, &sel); !errors.Is(err, ErrDirectedScrollUnsupported) {
		t.Fatal(err)
	}
	if len(f.events) != before {
		t.Fatal("precheck sent device input")
	}
	inputCursorPos = func() (int32, int32, error) { return 1<<31 - 1, 0, nil }
	if err := moveImpl(sel, 1, 1, 0, true); !errors.Is(err, errRelativeMoveOverflow) {
		t.Fatal(err)
	}
}

func TestPR1ConcurrentCommitsKeepKindDevicePaired(t *testing.T) {
	preservePR1Seams(t)
	callHIDInit = func() error { return nil }
	setGlobalBackend(BackendMessage)
	results := make(chan error, 4)
	for _, kind := range []Backend{BackendHID, 42, BackendMessage, -1} {
		kind := kind
		go func() {
			for i := 0; i < 100; i++ {
				if err := SetBackend(kind); err != nil {
					results <- err
					return
				}
				inputMutex.Lock()
				sel, err := acquireBackendImpl(nil)
				inputMutex.Unlock()
				if err != nil {
					results <- err
					return
				}
				if (sel.kind == BackendHID) != (sel.device != nil) {
					results <- fmt.Errorf("mismatched snapshot: %+v", sel)
					return
				}
			}
			results <- nil
		}()
	}
	for i := 0; i < 4; i++ {
		if err := receivePR1(t, results); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPR1FixedDeviceSnapshotSkipsRuntimePreparation(t *testing.T) {
	preservePR1Seams(t)
	calls := 0
	callHIDInit = func() error { calls++; return nil }
	in, err := NewInput(BackendHID)
	if err != nil || in.selection.kind != BackendHID || in.selection.device == nil {
		t.Fatalf("constructor: %+v %v", in, err)
	}
	f := &recordingBackend{}
	// Replace just the device transport, not acquisition, to observe the fixed
	// snapshot after a global switch and an injected initialization failure.
	in.selection.device = f
	if err := SetBackend(42); err != nil {
		t.Fatal(err)
	}
	callHIDInit = func() error { calls++; return errors.New("must not run") }
	if err := pressGlobal(KeyA, 0, &in.selection); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("fixed input reinitialized: %d", calls)
	}
	expectEvents(t, f.events, []string{"down:1e", "up:1e"})
}

func TestPR1TypeDeviceReleaseFailuresStopBeforeNextCharacter(t *testing.T) {
	preservePR1Seams(t)
	f := &recordingBackend{}
	first, second, shift := errors.New("first up"), errors.New("second up"), errors.New("shift up")
	upCalls := 0
	f.fail = func(event string) error {
		if event == "up:1e" {
			upCalls++
			if upCalls == 1 {
				return first
			}
			return second
		}
		if event == "up:2a" {
			return shift
		}
		return nil
	}
	deviceTypeSleep = func(time.Duration) {}
	sel := backendSelection{BackendHID, f}
	err := typeDevice(sel, "Aa", 0, true, 0)
	if !errors.Is(err, first) || !errors.Is(err, shift) || errors.Is(err, second) {
		t.Fatalf("cleanup matching changed: %v", err)
	}
	expectEvents(t, f.events, []string{"down:2a", "down:1e", "up:1e", "up:1e", "up:2a", "up:2a"})
}

func TestPR1TypeDeviceAllFourSendsUseKeySeams(t *testing.T) {
	preservePR1Seams(t)
	var events []string
	deviceTypeSleep = func(time.Duration) {}
	keyDown = func(sel backendSelection, hwnd uintptr, k Key) error {
		if hwnd != 0 || sel.device == nil {
			t.Error("Type send repeated target check or lost device")
		}
		events = append(events, fmt.Sprintf("down:%x", uint16(k)))
		return nil
	}
	keyUp = func(sel backendSelection, hwnd uintptr, k Key) error {
		if hwnd != 0 || sel.device == nil {
			t.Error("Type release lost snapshot")
		}
		events = append(events, fmt.Sprintf("up:%x", uint16(k)))
		return nil
	}
	if err := typeDevice(backendSelection{BackendHID, &recordingBackend{}}, "A", 0, true, 0); err != nil {
		t.Fatal(err)
	}
	expectEvents(t, events, []string{"down:2a", "down:1e", "up:1e", "up:2a"})
}
