package winput

import (
	"errors"
	"fmt"
	"sync"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/rpdg/winput/hid"
	"github.com/rpdg/winput/keyboard"
	"github.com/rpdg/winput/mouse"
	"github.com/rpdg/winput/screen"
	"github.com/rpdg/winput/uia"
	"github.com/rpdg/winput/window"
)

// Window represents a handle to a window.
type Window struct {
	HWND uintptr
}

// -----------------------------------------------------------------------------
// Window Discovery
// -----------------------------------------------------------------------------

// FindByTitle searches for a top-level window matching the exact title.
func FindByTitle(title string) (*Window, error) {
	hwnd, err := window.FindByTitle(title)
	if err != nil {
		return nil, ErrWindowNotFound
	}
	return &Window{HWND: hwnd}, nil
}

// FindByClass searches for a top-level window matching the specified class name.
func FindByClass(class string) (*Window, error) {
	hwnd, err := window.FindByClass(class)
	if err != nil {
		return nil, ErrWindowNotFound
	}
	return &Window{HWND: hwnd}, nil
}

// FindByPID returns all top-level windows belonging to the specified Process ID.
func FindByPID(pid uint32) ([]*Window, error) {
	hwnds, err := window.FindByPID(pid)
	if err != nil {
		return nil, ErrWindowNotFound
	}
	windows := make([]*Window, len(hwnds))
	for i, h := range hwnds {
		windows[i] = &Window{HWND: h}
	}
	return windows, nil
}

// FindByProcessName searches for all top-level windows belonging to a process with the given executable name.
func FindByProcessName(name string) ([]*Window, error) {
	pid, err := window.FindPIDByName(name)
	if err != nil {
		return nil, err
	}
	return FindByPID(pid)
}

// FindChildByClass searches for a child window with the specified class name.
func (w *Window) FindChildByClass(class string) (*Window, error) {
	hwnd, err := window.FindChildByClass(w.HWND, class)
	if err != nil {
		return nil, err
	}
	return &Window{HWND: hwnd}, nil
}

// Text returns the current text/value of the target window or control.
// It is most reliable for standard Win32 text controls such as Edit and RichEdit.
func (w *Window) Text() (string, error) {
	if !w.IsValid() {
		return "", ErrWindowGone
	}

	text, err := window.GetText(w.HWND)
	if err != nil {
		return "", ErrReadTextFailed
	}
	return text, nil
}

// Value returns the current best-effort textual value of the target window or control.
// It first tries Win32 text retrieval, then falls back to UI Automation for modern controls.
func (w *Window) Value() (string, error) {
	if !w.IsValid() {
		return "", ErrWindowGone
	}

	text, err := window.GetText(w.HWND)
	if err == nil && text != "" {
		return text, nil
	}

	text, err = uia.GetText(w.HWND)
	if err == nil {
		return text, nil
	}

	if text == "" {
		text, winErr := window.GetText(w.HWND)
		if winErr == nil {
			return text, nil
		}
	}

	return "", ErrReadTextFailed
}

// -----------------------------------------------------------------------------
// Window State
// -----------------------------------------------------------------------------

// IsValid checks if the window handle is valid.
func (w *Window) IsValid() bool {
	return window.IsValid(w.HWND)
}

// IsVisible checks if the window is visible and not minimized.
func (w *Window) IsVisible() bool {
	return window.IsVisible(w.HWND) && !window.IsIconic(w.HWND)
}

func (w *Window) checkReady() error {
	if !w.IsValid() {
		return ErrWindowGone
	}
	if !w.IsVisible() {
		return ErrWindowNotVisible
	}
	return nil
}

// -----------------------------------------------------------------------------
// Backend Configuration
// -----------------------------------------------------------------------------

// Backend represents the input simulation backend.
type Backend int

const (
	// BackendMessage uses Windows messages (PostMessage) for input simulation.
	BackendMessage Backend = iota
	// BackendHID uses the Interception driver for hardware-level input simulation.
	BackendHID
)

var (
	currentBackend Backend = BackendMessage
	backendMutex   sync.RWMutex
	inputMutex     sync.Mutex
)

// SetBackend sets the input simulation backend.
// If BackendHID is selected, it attempts to initialize the Interception driver immediately.
// Returns an error if the driver or DLL cannot be loaded.
func SetBackend(b Backend) error {
	backendMutex.Lock()
	defer backendMutex.Unlock()

	if b == BackendHID {
		// Eager initialization: Fail fast if driver/DLL is missing
		if err := hid.Init(); err != nil {
			if errors.Is(err, hid.ErrDriverNotInstalled) {
				return ErrDriverNotInstalled
			}
			return fmt.Errorf("%w: %v", ErrDLLLoadFailed, err)
		}
	}

	currentBackend = b
	return nil
}

// SetHIDLibraryPath sets the path to the interception.dll library.
func SetHIDLibraryPath(path string) {
	hid.SetLibraryPath(path)
}

func checkBackend() error {
	backendMutex.RLock()
	cb := currentBackend
	backendMutex.RUnlock()

	if cb == BackendHID {
		if err := hid.Init(); err != nil {
			if errors.Is(err, hid.ErrDriverNotInstalled) {
				return ErrDriverNotInstalled
			}
			return fmt.Errorf("%w: %v", ErrDLLLoadFailed, err)
		}
	}
	return nil
}

func getBackend() Backend {
	backendMutex.RLock()
	defer backendMutex.RUnlock()
	return currentBackend
}

// -----------------------------------------------------------------------------
// Implementation Helpers (No Lock)
// -----------------------------------------------------------------------------

func moveImpl(cb Backend, hwnd uintptr, x, y int32, isRelative bool) error {
	if cb == BackendHID {
		if isRelative {
			cx, cy, err := window.GetCursorPos()
			if err != nil {
				return err
			}
			return hid.Move(cx+x, cy+y)
		} else {
			sx, sy, err := window.ClientToScreen(hwnd, x, y)
			if err != nil {
				return err
			}
			return hid.Move(sx, sy)
		}
	}

	if isRelative {
		sx, sy, err := window.GetCursorPos()
		if err != nil {
			return err
		}
		tx, ty := sx+x, sy+y
		cx, cy, err := window.ScreenToClient(hwnd, tx, ty)
		if err != nil {
			return err
		}
		return mouse.Move(hwnd, cx, cy)
	}
	return mouse.Move(hwnd, x, y)
}

func keyDownImpl(cb Backend, hwnd uintptr, k Key) error {
	if cb == BackendHID {
		return hid.KeyDown(uint16(k))
	}
	if hwnd == 0 {
		return sendScanCode(k, false)
	}
	return keyboard.KeyDown(hwnd, k)
}

func keyUpImpl(cb Backend, hwnd uintptr, k Key) error {
	if cb == BackendHID {
		return hid.KeyUp(uint16(k))
	}
	if hwnd == 0 {
		return sendScanCode(k, true)
	}
	return keyboard.KeyUp(hwnd, k)
}

func isModifierKey(k Key) bool {
	switch k {
	case KeyCtrl, KeyAlt, KeyShift, KeyRightCtrl, KeyRightAlt, KeyRightShift:
		return true
	default:
		return false
	}
}

func hotkeyNeedsForeground(keys []Key) bool {
	for _, k := range keys {
		if isModifierKey(k) {
			return true
		}
	}
	return false
}

func validateKey(cb Backend, hwnd uintptr, k Key) error {
	if _, _, ok := k.Decode(); !ok {
		return ErrUnsupportedKey
	}
	if cb == BackendMessage && hwnd != 0 {
		if _, ok := keyboard.VirtualKey(k); !ok {
			return ErrUnsupportedKey
		}
	}
	return nil
}

func validateKeys(cb Backend, hwnd uintptr, keys []Key) error {
	for _, k := range keys {
		if err := validateKey(cb, hwnd, k); err != nil {
			return err
		}
	}
	return nil
}

// inputScope records the release closures for inputs that were successfully
// pressed. Release runs them in reverse order, retrying each failed release
// once.
type inputScope struct {
	releases []func() error
}

// Press runs down and records up only after down succeeds.
func (s *inputScope) Press(down, up func() error) error {
	if err := down(); err != nil {
		return err
	}
	s.releases = append(s.releases, up)
	return nil
}

// Release runs every recorded release in reverse order, even when one fails.
func (s *inputScope) Release() error {
	var errs []error
	for i := len(s.releases) - 1; i >= 0; i-- {
		release := s.releases[i]
		if err := release(); err != nil {
			if err2 := release(); err2 != nil {
				errs = append(errs, err)
			}
		}
	}
	s.releases = nil
	return errors.Join(errs...)
}

func pressHotkeyForeground(hwnd uintptr, keys []Key) (err error) {
	if err := validateKeys(BackendMessage, hwnd, keys); err != nil {
		return err
	}

	prev := window.GetForegroundWindow()
	if prev != hwnd {
		// Once a foreground switch is attempted, every error exit restores prev.
		defer func() {
			if prev != 0 && prev != hwnd {
				if restoreErr := window.ForceSetForegroundWindow(prev); restoreErr != nil {
					err = errors.Join(err, fmt.Errorf("%w: failed to restore foreground window: %v", ErrPermissionDenied, restoreErr))
				}
			}
		}()

		if err := window.ForceSetForegroundWindow(hwnd); err != nil {
			return fmt.Errorf("%w: failed to foreground target window for modifier hotkey: %v", ErrPermissionDenied, err)
		}
		if window.GetForegroundWindow() != hwnd {
			return fmt.Errorf("%w: target window did not become foreground (ASFW restriction)", ErrPermissionDenied)
		}
		time.Sleep(50 * time.Millisecond)
	}

	var scope inputScope
	defer func() {
		err = errors.Join(err, scope.Release())
	}()

	for _, k := range keys {
		key := k
		if err := scope.Press(
			func() error { return sendScanCode(key, false) },
			func() error { return sendScanCode(key, true) },
		); err != nil {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	return nil
}

// -----------------------------------------------------------------------------
// Input API (Mouse)
// -----------------------------------------------------------------------------

// Move simulates mouse movement to the specified client coordinates.
func (w *Window) Move(x, y int32) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := w.checkReady(); err != nil {
		return err
	}
	if err := checkBackend(); err != nil {
		return err
	}
	return moveImpl(getBackend(), w.HWND, x, y, false)
}

// MoveRel simulates relative mouse movement from the current cursor position.
func (w *Window) MoveRel(dx, dy int32) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := w.checkReady(); err != nil {
		return err
	}
	if err := checkBackend(); err != nil {
		return err
	}
	return moveImpl(getBackend(), w.HWND, dx, dy, true)
}

// Click simulates a left mouse button click at the specified client coordinates.
func (w *Window) Click(x, y int32) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := w.checkReady(); err != nil {
		return err
	}
	if err := checkBackend(); err != nil {
		return err
	}

	if getBackend() == BackendHID {
		sx, sy, err := window.ClientToScreen(w.HWND, x, y)
		if err != nil {
			return err
		}
		return hid.Click(sx, sy)
	}
	return mouse.Click(w.HWND, x, y)
}

// ClickRight simulates a right mouse button click at the specified client coordinates.
func (w *Window) ClickRight(x, y int32) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := w.checkReady(); err != nil {
		return err
	}
	if err := checkBackend(); err != nil {
		return err
	}

	if getBackend() == BackendHID {
		sx, sy, err := window.ClientToScreen(w.HWND, x, y)
		if err != nil {
			return err
		}
		return hid.ClickRight(sx, sy)
	}
	return mouse.ClickRight(w.HWND, x, y)
}

// ClickMiddle simulates a middle mouse button click at the specified client coordinates.
func (w *Window) ClickMiddle(x, y int32) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := w.checkReady(); err != nil {
		return err
	}
	if err := checkBackend(); err != nil {
		return err
	}

	if getBackend() == BackendHID {
		sx, sy, err := window.ClientToScreen(w.HWND, x, y)
		if err != nil {
			return err
		}
		return hid.ClickMiddle(sx, sy)
	}
	return mouse.ClickMiddle(w.HWND, x, y)
}

// DoubleClick simulates a left mouse button double-click at the specified client coordinates.
func (w *Window) DoubleClick(x, y int32) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := w.checkReady(); err != nil {
		return err
	}
	if err := checkBackend(); err != nil {
		return err
	}

	if getBackend() == BackendHID {
		sx, sy, err := window.ClientToScreen(w.HWND, x, y)
		if err != nil {
			return err
		}
		return hid.DoubleClick(sx, sy)
	}
	return mouse.DoubleClick(w.HWND, x, y)
}

// Scroll simulates a vertical mouse wheel scroll at the specified client coordinates.
//
// Under BackendMessage the WM_MOUSEWHEEL message is posted to this window.
// Under BackendHID this method returns ErrDirectedScrollUnsupported because a
// hardware wheel event cannot be addressed to a specific window; use
// ScrollWheel for hardware-level scrolling.
func (w *Window) Scroll(x, y int32, delta int32) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := w.checkReady(); err != nil {
		return err
	}
	if err := checkBackend(); err != nil {
		return err
	}

	if getBackend() == BackendHID {
		return ErrDirectedScrollUnsupported
	}
	return mouse.Scroll(w.HWND, x, y, delta)
}

// -----------------------------------------------------------------------------
// Global Input API (Screen Coordinates)
// -----------------------------------------------------------------------------

// MoveMouseTo moves the mouse cursor to the specified absolute screen coordinates (Virtual Desktop).
func MoveMouseTo(x, y int32) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := checkBackend(); err != nil {
		return err
	}

	if getBackend() == BackendHID {
		return hid.Move(x, y)
	}

	r, _, _ := window.ProcSetCursorPos.Call(uintptr(x), uintptr(y))
	if r == 0 {
		return fmt.Errorf("SetCursorPos failed")
	}
	return nil
}

// ClickMouseAt moves to the specified screen coordinates and performs a left click.
//
// Under BackendMessage the click uses mouse_event, which has no return value;
// delivery failures of the button events are therefore not observable.
func ClickMouseAt(x, y int32) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := checkBackend(); err != nil {
		return err
	}

	if getBackend() == BackendHID {
		return hid.Click(x, y)
	}

	// Message Backend Fallback (duplicated logic from MoveMouseTo to avoid calling locked func)
	r, _, _ := window.ProcSetCursorPos.Call(uintptr(x), uintptr(y))
	if r == 0 {
		return fmt.Errorf("SetCursorPos failed")
	}

	time.Sleep(30 * time.Millisecond)
	window.ProcMouseEvent.Call(0x0002, 0, 0, 0, 0)
	window.ProcMouseEvent.Call(0x0004, 0, 0, 0, 0)
	return nil
}

// DoubleClickMouseAt moves to the specified screen coordinates and performs a left double-click.
//
// Under BackendMessage the clicks use mouse_event, which has no return value;
// delivery failures of the button events are therefore not observable.
func DoubleClickMouseAt(x, y int32) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := checkBackend(); err != nil {
		return err
	}

	if getBackend() == BackendHID {
		return hid.DoubleClick(x, y)
	}

	// Message Backend Fallback
	r, _, _ := window.ProcSetCursorPos.Call(uintptr(x), uintptr(y))
	if r == 0 {
		return fmt.Errorf("SetCursorPos failed")
	}

	// Double click = Down -> Up -> (delay) -> Down -> Up
	// But standard mouse_event doesn't have DBLCLK. We just click twice fast.
	const (
		MOUSEEVENTF_LEFTDOWN = 0x0002
		MOUSEEVENTF_LEFTUP   = 0x0004
	)

	// First Click
	time.Sleep(30 * time.Millisecond)
	window.ProcMouseEvent.Call(MOUSEEVENTF_LEFTDOWN, 0, 0, 0, 0)
	window.ProcMouseEvent.Call(MOUSEEVENTF_LEFTUP, 0, 0, 0, 0)

	// Interval (short enough for OS to register as double click)
	time.Sleep(50 * time.Millisecond)

	// Second Click
	window.ProcMouseEvent.Call(MOUSEEVENTF_LEFTDOWN, 0, 0, 0, 0)
	window.ProcMouseEvent.Call(MOUSEEVENTF_LEFTUP, 0, 0, 0, 0)

	return nil
}

// ClickRightMouseAt moves to the specified screen coordinates and performs a right click.
//
// Under BackendMessage the click uses mouse_event, which has no return value;
// delivery failures of the button events are therefore not observable.
func ClickRightMouseAt(x, y int32) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := checkBackend(); err != nil {
		return err
	}

	if getBackend() == BackendHID {
		return hid.ClickRight(x, y)
	}

	r, _, _ := window.ProcSetCursorPos.Call(uintptr(x), uintptr(y))
	if r == 0 {
		return fmt.Errorf("SetCursorPos failed")
	}

	time.Sleep(30 * time.Millisecond)
	window.ProcMouseEvent.Call(0x0008, 0, 0, 0, 0) // RIGHTDOWN
	window.ProcMouseEvent.Call(0x0010, 0, 0, 0, 0) // RIGHTUP
	return nil
}

// ClickMiddleMouseAt moves to the specified screen coordinates and performs a middle click.
//
// Under BackendMessage the click uses mouse_event, which has no return value;
// delivery failures of the button events are therefore not observable.
func ClickMiddleMouseAt(x, y int32) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := checkBackend(); err != nil {
		return err
	}

	if getBackend() == BackendHID {
		return hid.ClickMiddle(x, y)
	}

	r, _, _ := window.ProcSetCursorPos.Call(uintptr(x), uintptr(y))
	if r == 0 {
		return fmt.Errorf("SetCursorPos failed")
	}

	time.Sleep(30 * time.Millisecond)
	window.ProcMouseEvent.Call(0x0020, 0, 0, 0, 0) // MIDDLEDOWN
	window.ProcMouseEvent.Call(0x0040, 0, 0, 0, 0) // MIDDLEUP
	return nil
}

// -----------------------------------------------------------------------------
// Mouse Button State and Exact Movement
// -----------------------------------------------------------------------------

// MouseButton identifies a mouse button independent of driver constants.
type MouseButton uint8

const (
	MouseLeft MouseButton = iota + 1
	MouseRight
	MouseMiddle
)

func hidButtonStates(button MouseButton) (down, up uint16, ok bool) {
	switch button {
	case MouseLeft:
		return 0x001, 0x002, true
	case MouseRight:
		return 0x004, 0x008, true
	case MouseMiddle:
		return 0x010, 0x020, true
	default:
		return 0, 0, false
	}
}

func mouseEventButtonFlags(button MouseButton, down bool) (uintptr, bool) {
	switch button {
	case MouseLeft:
		if down {
			return 0x0002, true
		}
		return 0x0004, true
	case MouseRight:
		if down {
			return 0x0008, true
		}
		return 0x0010, true
	case MouseMiddle:
		if down {
			return 0x0020, true
		}
		return 0x0040, true
	default:
		return 0, false
	}
}

func mouseButtonImpl(cb Backend, button MouseButton, down bool) error {
	if cb == BackendHID {
		downState, upState, ok := hidButtonStates(button)
		if !ok {
			return ErrInvalidMouseButton
		}
		state := downState
		if !down {
			state = upState
		}
		return hid.Button(state)
	}

	flags, ok := mouseEventButtonFlags(button, down)
	if !ok {
		return ErrInvalidMouseButton
	}
	window.ProcMouseEvent.Call(flags, 0, 0, 0, 0)
	return nil
}

// MouseDown presses the specified button at the current cursor position.
// The input mutex covers this call only; a down/move/up sequence across
// separate calls is not atomic.
func MouseDown(button MouseButton) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := checkBackend(); err != nil {
		return err
	}
	return mouseButtonImpl(getBackend(), button, true)
}

// MouseUp releases the specified button at the current cursor position.
// The input mutex covers this call only; a down/move/up sequence across
// separate calls is not atomic.
func MouseUp(button MouseButton) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := checkBackend(); err != nil {
		return err
	}
	return mouseButtonImpl(getBackend(), button, false)
}

// ClickMouse performs a left click at the current cursor position without moving.
//
// Under BackendMessage the click uses mouse_event, which has no return value;
// delivery failures of the button events are therefore not observable.
func ClickMouse() error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := checkBackend(); err != nil {
		return err
	}

	if getBackend() == BackendHID {
		if err := hid.Button(0x001); err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
		if err := hid.Button(0x002); err != nil {
			if err2 := hid.Button(0x002); err2 != nil {
				return err
			}
		}
		return nil
	}

	window.ProcMouseEvent.Call(0x0002, 0, 0, 0, 0)
	time.Sleep(50 * time.Millisecond)
	window.ProcMouseEvent.Call(0x0004, 0, 0, 0, 0)
	return nil
}

func addCursorDelta(cx, cy, dx, dy int32) (int32, int32, bool) {
	nx := int64(cx) + int64(dx)
	ny := int64(cy) + int64(dy)
	if nx < -1<<31 || nx > 1<<31-1 || ny < -1<<31 || ny > 1<<31-1 {
		return 0, 0, false
	}
	return int32(nx), int32(ny), true
}

// MoveMouseBy moves the cursor by the specified delta.
//
// The HID backend sends one relative stroke. The message backend reads the
// cursor, adds the delta using int64, and calls SetCursorPos; success means
// the system accepted the resulting coordinate.
func MoveMouseBy(dx, dy int32) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := checkBackend(); err != nil {
		return err
	}

	if getBackend() == BackendHID {
		return hid.MoveRelative(dx, dy)
	}

	cx, cy, err := window.GetCursorPos()
	if err != nil {
		return err
	}
	nx, ny, ok := addCursorDelta(cx, cy, dx, dy)
	if !ok {
		return fmt.Errorf("relative move result exceeds int32")
	}
	r, _, _ := window.ProcSetCursorPos.Call(uintptr(nx), uintptr(ny))
	if r == 0 {
		return fmt.Errorf("SetCursorPos failed")
	}
	return nil
}

func withinVirtualDesktop(x, y int32, bounds screen.Rect) bool {
	return x >= bounds.Left && x < bounds.Right && y >= bounds.Top && y < bounds.Bottom
}

// MoveMouseToExact performs one absolute move.
//
// The HID path sends one absolute stroke over the virtual desktop, waits 12ms,
// and verifies the cursor within one pixel using the process coordinate space.
// That reading is not a physical-pixel proof (see RISK-001 in the input
// review). The message path calls SetCursorPos; success means the system
// accepted it.
func MoveMouseToExact(x, y int32) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := checkBackend(); err != nil {
		return err
	}

	bounds := screen.VirtualBounds()
	width := int64(bounds.Right - bounds.Left)
	height := int64(bounds.Bottom - bounds.Top)
	if width < 2 || height < 2 {
		return fmt.Errorf("virtual desktop too small for exact movement")
	}
	if !withinVirtualDesktop(x, y, bounds) {
		return ErrOutsideVirtualDesktop
	}

	if getBackend() == BackendHID {
		return hid.MoveExact(x, y)
	}

	r, _, _ := window.ProcSetCursorPos.Call(uintptr(x), uintptr(y))
	if r == 0 {
		return fmt.Errorf("SetCursorPos failed")
	}
	return nil
}

// ScrollWheel scrolls by whole wheel steps. Positive steps scroll forward.
// Zero and values outside [-273, 273] are rejected before multiplication.
func ScrollWheel(steps int) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := checkBackend(); err != nil {
		return err
	}
	if steps == 0 || steps < -273 || steps > 273 {
		return ErrInvalidScrollSteps
	}

	delta := int32(steps * 120)
	if getBackend() == BackendHID {
		return hid.Scroll(delta)
	}

	// mouse_event(MOUSEEVENTF_WHEEL, 0, 0, delta, 0)
	window.ProcMouseEvent.Call(0x0800, 0, uintptr(uint32(delta)), 0, 0)
	return nil
}

// PressFor holds a global key for at least 10ms.
func PressFor(key Key, duration time.Duration) (err error) {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := checkBackend(); err != nil {
		return err
	}
	if duration < 10*time.Millisecond {
		return ErrInvalidPressDuration
	}

	cb := getBackend()
	if err := validateKey(cb, 0, key); err != nil {
		return err
	}

	var scope inputScope
	defer func() {
		err = errors.Join(err, scope.Release())
	}()
	if err := scope.Press(
		func() error { return keyDownImpl(cb, 0, key) },
		func() error { return keyUpImpl(cb, 0, key) },
	); err != nil {
		return err
	}
	time.Sleep(duration)
	return nil
}

// -----------------------------------------------------------------------------
// Input API (Keyboard)
// -----------------------------------------------------------------------------

type Key = keyboard.Key

const (
	KeyEsc            = keyboard.KeyEsc
	Key1              = keyboard.Key1
	Key2              = keyboard.Key2
	Key3              = keyboard.Key3
	Key4              = keyboard.Key4
	Key5              = keyboard.Key5
	Key6              = keyboard.Key6
	Key7              = keyboard.Key7
	Key8              = keyboard.Key8
	Key9              = keyboard.Key9
	Key0              = keyboard.Key0
	KeyMinus          = keyboard.KeyMinus
	KeyEqual          = keyboard.KeyEqual
	KeyBkSp           = keyboard.KeyBkSp
	KeyTab            = keyboard.KeyTab
	KeyQ              = keyboard.KeyQ
	KeyW              = keyboard.KeyW
	KeyE              = keyboard.KeyE
	KeyR              = keyboard.KeyR
	KeyT              = keyboard.KeyT
	KeyY              = keyboard.KeyY
	KeyU              = keyboard.KeyU
	KeyI              = keyboard.KeyI
	KeyO              = keyboard.KeyO
	KeyP              = keyboard.KeyP
	KeyLBr            = keyboard.KeyLBr
	KeyRBr            = keyboard.KeyRBr
	KeyEnter          = keyboard.KeyEnter
	KeyLeftCtrl       = keyboard.KeyLeftCtrl
	KeyCtrl           = keyboard.KeyCtrl
	KeyRightCtrl      = keyboard.KeyRightCtrl
	KeyA              = keyboard.KeyA
	KeyS              = keyboard.KeyS
	KeyD              = keyboard.KeyD
	KeyF              = keyboard.KeyF
	KeyG              = keyboard.KeyG
	KeyH              = keyboard.KeyH
	KeyJ              = keyboard.KeyJ
	KeyK              = keyboard.KeyK
	KeyL              = keyboard.KeyL
	KeySemi           = keyboard.KeySemi
	KeyQuot           = keyboard.KeyQuot
	KeyTick           = keyboard.KeyTick
	KeyLeftShift      = keyboard.KeyLeftShift
	KeyShift          = keyboard.KeyShift
	KeyRightShift     = keyboard.KeyRightShift
	KeyBackslash      = keyboard.KeyBackslash
	KeyZ              = keyboard.KeyZ
	KeyX              = keyboard.KeyX
	KeyC              = keyboard.KeyC
	KeyV              = keyboard.KeyV
	KeyB              = keyboard.KeyB
	KeyN              = keyboard.KeyN
	KeyM              = keyboard.KeyM
	KeyComma          = keyboard.KeyComma
	KeyDot            = keyboard.KeyDot
	KeySlash          = keyboard.KeySlash
	KeyNumpadMultiply = keyboard.KeyNumpadMultiply
	KeyLeftAlt        = keyboard.KeyLeftAlt
	KeyAlt            = keyboard.KeyAlt
	KeyRightAlt       = keyboard.KeyRightAlt
	KeySpace          = keyboard.KeySpace
	KeyCaps           = keyboard.KeyCaps
	KeyF1             = keyboard.KeyF1
	KeyF2             = keyboard.KeyF2
	KeyF3             = keyboard.KeyF3
	KeyF4             = keyboard.KeyF4
	KeyF5             = keyboard.KeyF5
	KeyF6             = keyboard.KeyF6
	KeyF7             = keyboard.KeyF7
	KeyF8             = keyboard.KeyF8
	KeyF9             = keyboard.KeyF9
	KeyF10            = keyboard.KeyF10
	KeyF11            = keyboard.KeyF11
	KeyF12            = keyboard.KeyF12
	KeyNumLock        = keyboard.KeyNumLock
	KeyScroll         = keyboard.KeyScroll

	KeyHome      = keyboard.KeyHome
	KeyArrowUp   = keyboard.KeyArrowUp
	KeyPageUp    = keyboard.KeyPageUp
	KeyLeft      = keyboard.KeyLeft
	KeyRight     = keyboard.KeyRight
	KeyEnd       = keyboard.KeyEnd
	KeyArrowDown = keyboard.KeyArrowDown
	KeyPageDown  = keyboard.KeyPageDown
	KeyInsert    = keyboard.KeyInsert
	KeyDelete    = keyboard.KeyDelete

	KeyNumpad0        = keyboard.KeyNumpad0
	KeyNumpad1        = keyboard.KeyNumpad1
	KeyNumpad2        = keyboard.KeyNumpad2
	KeyNumpad3        = keyboard.KeyNumpad3
	KeyNumpad4        = keyboard.KeyNumpad4
	KeyNumpad5        = keyboard.KeyNumpad5
	KeyNumpad6        = keyboard.KeyNumpad6
	KeyNumpad7        = keyboard.KeyNumpad7
	KeyNumpad8        = keyboard.KeyNumpad8
	KeyNumpad9        = keyboard.KeyNumpad9
	KeyNumpadDecimal  = keyboard.KeyNumpadDecimal
	KeyNumpadAdd      = keyboard.KeyNumpadAdd
	KeyNumpadSubtract = keyboard.KeyNumpadSubtract
	KeyNumpadDivide   = keyboard.KeyNumpadDivide
	KeyDivide         = keyboard.KeyDivide
	KeyNumpadEnter    = keyboard.KeyNumpadEnter
	KeyLWin           = keyboard.KeyLWin
	KeyRWin           = keyboard.KeyRWin
)

// KeyFromRune attempts to map a unicode character to a Key.
func KeyFromRune(r rune) (Key, bool) {
	k, _, ok := keyboard.LookupKey(r)
	return k, ok
}

// Public Wrappers using Lock

// KeyDown sends a key down event to the window.
func (w *Window) KeyDown(key Key) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := w.checkReady(); err != nil {
		return err
	}
	if err := checkBackend(); err != nil {
		return err
	}
	return keyDownImpl(getBackend(), w.HWND, key)
}

// KeyUp sends a key up event to the window.
func (w *Window) KeyUp(key Key) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := w.checkReady(); err != nil {
		return err
	}
	if err := checkBackend(); err != nil {
		return err
	}
	return keyUpImpl(getBackend(), w.HWND, key)
}

// Press simulates a key press (down then up).
func (w *Window) Press(key Key) (err error) {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := w.checkReady(); err != nil {
		return err
	}
	if err := checkBackend(); err != nil {
		return err
	}

	cb := getBackend()
	if err := validateKey(cb, w.HWND, key); err != nil {
		return err
	}

	var scope inputScope
	defer func() {
		err = errors.Join(err, scope.Release())
	}()
	if err := scope.Press(
		func() error { return keyDownImpl(cb, w.HWND, key) },
		func() error { return keyUpImpl(cb, w.HWND, key) },
	); err != nil {
		return err
	}
	time.Sleep(30 * time.Millisecond)
	return nil
}

// PressHotkey presses a combination of keys (e.g., Ctrl+A).
// Under BackendMessage, modifier combinations (Ctrl/Alt/Shift + another key)
// are promoted to foreground keyboard events because PostMessage alone does
// not provide reliable modifier state to many target windows.
func (w *Window) PressHotkey(keys ...Key) (err error) {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := w.checkReady(); err != nil {
		return err
	}
	if err := checkBackend(); err != nil {
		return err
	}

	cb := getBackend()
	if err := validateKeys(cb, w.HWND, keys); err != nil {
		return err
	}
	if cb == BackendMessage && w.HWND != 0 && hotkeyNeedsForeground(keys) {
		return pressHotkeyForeground(w.HWND, keys)
	}

	var scope inputScope
	defer func() {
		err = errors.Join(err, scope.Release())
	}()
	for _, k := range keys {
		key := k
		if err := scope.Press(
			func() error { return keyDownImpl(cb, w.HWND, key) },
			func() error { return keyUpImpl(cb, w.HWND, key) },
		); err != nil {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	return nil
}

// Type simulates typing text.
func (w *Window) Type(text string) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := w.checkReady(); err != nil {
		return err
	}
	if err := checkBackend(); err != nil {
		return err
	}

	cb := getBackend()
	if cb == BackendMessage {
		// Use WM_CHAR for reliability in background
		return keyboard.Type(w.HWND, text)
	}

	return typeHID(text)
}

// typeHID types text with the HID backend. The whole string is validated
// before the first key is pressed; each character's Shift state is released
// before the next character starts.
func typeHID(text string) error {
	for _, r := range text {
		if _, _, ok := keyboard.LookupKey(r); !ok {
			return ErrUnsupportedKey
		}
	}

	for _, r := range text {
		k, shifted, _ := keyboard.LookupKey(r)
		scope := &inputScope{}
		err := func() (err error) {
			defer func() {
				err = errors.Join(err, scope.Release())
			}()
			if shifted {
				if err := scope.Press(
					func() error { return hid.KeyDown(uint16(KeyShift)) },
					func() error { return hid.KeyUp(uint16(KeyShift)) },
				); err != nil {
					return err
				}
				time.Sleep(10 * time.Millisecond)
			}
			if err := scope.Press(
				func() error { return hid.KeyDown(uint16(k)) },
				func() error { return hid.KeyUp(uint16(k)) },
			); err != nil {
				return err
			}
			time.Sleep(30 * time.Millisecond)
			return nil
		}()
		if err != nil {
			return err
		}
	}
	return nil
}

// Global Wrappers

// KeyDown simulates a global key down event.
func KeyDown(k Key) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := checkBackend(); err != nil {
		return err
	}
	return keyDownImpl(getBackend(), 0, k)
}

// KeyUp simulates a global key up event.
func KeyUp(k Key) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := checkBackend(); err != nil {
		return err
	}
	return keyUpImpl(getBackend(), 0, k)
}

// Press simulates a global key press (down then up).
func Press(k Key) (err error) {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := checkBackend(); err != nil {
		return err
	}

	cb := getBackend()
	if err := validateKey(cb, 0, k); err != nil {
		return err
	}

	var scope inputScope
	defer func() {
		err = errors.Join(err, scope.Release())
	}()
	if err := scope.Press(
		func() error { return keyDownImpl(cb, 0, k) },
		func() error { return keyUpImpl(cb, 0, k) },
	); err != nil {
		return err
	}
	time.Sleep(30 * time.Millisecond)
	return nil
}

// PressHotkey simulates a global combination of keys.
func PressHotkey(keys ...Key) (err error) {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := checkBackend(); err != nil {
		return err
	}

	cb := getBackend()
	if err := validateKeys(cb, 0, keys); err != nil {
		return err
	}

	var scope inputScope
	defer func() {
		err = errors.Join(err, scope.Release())
	}()
	for _, k := range keys {
		key := k
		if err := scope.Press(
			func() error { return keyDownImpl(cb, 0, key) },
			func() error { return keyUpImpl(cb, 0, key) },
		); err != nil {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	return nil
}

// Type simulates typing text globally.
func Type(text string) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := checkBackend(); err != nil {
		return err
	}

	cb := getBackend()
	if cb == BackendHID {
		return typeHID(text)
	}

	for _, r := range text {
		if err := sendUnicode(r); err != nil {
			return err
		}
		time.Sleep(30 * time.Millisecond)
	}
	return nil
}

// Internal structures for SendInput. The union is MOUSEINPUT, the largest
// member; KEYBDINPUT is written at the start of it.
type keyboardInput struct {
	WVk         uint16
	WScan       uint16
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

type mouseInput struct {
	Dx          int32
	Dy          int32
	MouseData   uint32
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

type input struct {
	Type uint32
	Mi   mouseInput
}

const (
	INPUT_KEYBOARD        = 1
	KEYEVENTF_EXTENDEDKEY = 0x0001
	KEYEVENTF_KEYUP       = 0x0002
	KEYEVENTF_UNICODE     = 0x0004
	KEYEVENTF_SCANCODE    = 0x0008
)

func scanCodeInput(k Key, up bool) (input, bool) {
	scan, extended, ok := k.Decode()
	if !ok {
		return input{}, false
	}

	var in input
	in.Type = INPUT_KEYBOARD
	ki := (*keyboardInput)(unsafe.Pointer(&in.Mi))
	ki.WScan = uint16(scan)

	flags := uint32(KEYEVENTF_SCANCODE)
	if extended {
		flags |= KEYEVENTF_EXTENDEDKEY
	}
	if up {
		flags |= KEYEVENTF_KEYUP
	}
	ki.DwFlags = flags
	return in, true
}

func sendScanCode(k Key, up bool) error {
	in, ok := scanCodeInput(k, up)
	if !ok {
		return ErrUnsupportedKey
	}

	n, _, _ := window.ProcSendInput.Call(1, uintptr(unsafe.Pointer(&in)), uintptr(unsafe.Sizeof(in)))
	if n != 1 {
		return ErrSendInputFailed
	}
	return nil
}

func makeUnicodeInput(unit uint16, up bool) input {
	var in input
	in.Type = INPUT_KEYBOARD
	ki := (*keyboardInput)(unsafe.Pointer(&in.Mi))
	ki.WScan = unit
	ki.DwFlags = KEYEVENTF_UNICODE
	if up {
		ki.DwFlags |= KEYEVENTF_KEYUP
	}
	return in
}

func unicodeUnitInputs(unit uint16) [2]input {
	return [2]input{
		makeUnicodeInput(unit, false),
		makeUnicodeInput(unit, true),
	}
}

// unicodeInputs returns the two events for each UTF-16 code unit. A non-BMP
// rune therefore produces four events: high surrogate down/up, low surrogate
// down/up.
func unicodeInputs(r rune) []input {
	units := utf16.Encode([]rune{r})
	inputs := make([]input, 0, len(units)*2)
	for _, unit := range units {
		pair := unicodeUnitInputs(unit)
		inputs = append(inputs, pair[:]...)
	}
	return inputs
}

func sendUnicode(r rune) error {
	for _, unit := range utf16.Encode([]rune{r}) {
		pair := unicodeUnitInputs(unit)
		n, _, _ := window.ProcSendInput.Call(2, uintptr(unsafe.Pointer(&pair[0])), uintptr(unsafe.Sizeof(pair[0])))
		switch n {
		case 2:
			continue
		case 1:
			// Only the key-down event was inserted. Repair it once.
			up := makeUnicodeInput(unit, true)
			n2, _, _ := window.ProcSendInput.Call(1, uintptr(unsafe.Pointer(&up)), uintptr(unsafe.Sizeof(up)))
			if n2 == 1 {
				return ErrSendInputPartial
			}
			return ErrSendInputStuck
		default:
			return ErrSendInputFailed
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// Coordinate & DPI
// -----------------------------------------------------------------------------

// GetCursorPos returns the current cursor position in screen coordinates.
func GetCursorPos() (int32, int32, error) {
	return window.GetCursorPos()
}

// EnablePerMonitorDPI sets the process to be Per-Monitor DPI aware.
func EnablePerMonitorDPI() error {
	return window.EnablePerMonitorDPI()
}

// DPI returns the DPI of the window.
func (w *Window) DPI() (uint32, uint32, error) {
	return window.GetDPI(w.HWND)
}

// ClientRect returns the client area dimensions of the window.
func (w *Window) ClientRect() (width, height int32, err error) {
	return window.GetClientRect(w.HWND)
}

// ScreenToClient converts screen coordinates to client coordinates.
func (w *Window) ScreenToClient(x, y int32) (cx, cy int32, err error) {
	return window.ScreenToClient(w.HWND, x, y)
}

// ClientToScreen converts client coordinates to screen coordinates.
func (w *Window) ClientToScreen(x, y int32) (sx, sy int32, err error) {
	return window.ClientToScreen(w.HWND, x, y)
}
