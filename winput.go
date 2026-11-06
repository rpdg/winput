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
		return nil, lookupError(err)
	}
	return &Window{HWND: hwnd}, nil
}

// FindByClass searches for a top-level window matching the specified class name.
func FindByClass(class string) (*Window, error) {
	hwnd, err := window.FindByClass(class)
	if err != nil {
		return nil, lookupError(err)
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

// FindByProcessName returns every top-level window belonging to any process
// whose executable name matches. Several processes may share that name; a
// process with no top-level window is skipped. Snapshot order is preserved.
func FindByProcessName(name string) ([]*Window, error) {
	pids, err := window.FindPIDsByName(name)
	if err != nil {
		return nil, err
	}
	return windowsForPIDs(pids, FindByPID)
}

// lookupError keeps ErrWindowNotFound for an ordinary miss. An embedded NUL
// is also that sentinel, and it keeps ErrInvalidLookupString so callers can
// tell a refused string from a window that is not there.
func lookupError(err error) error {
	if errors.Is(err, window.ErrInvalidLookupString) {
		return fmt.Errorf("%w: %w", ErrWindowNotFound, err)
	}
	return ErrWindowNotFound
}

// windowsForPIDs collects lookup results for every PID. A PID that has no
// windows does not fail the whole call. The result is ErrWindowNotFound only
// when every PID contributes nothing.
func windowsForPIDs(pids []uint32, lookup func(uint32) ([]*Window, error)) ([]*Window, error) {
	var all []*Window
	for _, pid := range pids {
		wins, err := lookup(pid)
		if err != nil || len(wins) == 0 {
			continue
		}
		all = append(all, wins...)
	}
	if len(all) == 0 {
		return nil, ErrWindowNotFound
	}
	return all, nil
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
// callHIDInit is the one HID initialization entry shared by SetBackend,
// checkBackend, ProbeHID, and NewInput(BackendHID). ProbeTarget must not call it.
// Failure is not cached; a later call may succeed.
var callHIDInit = hid.Init

func translateHIDInit(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, hid.ErrDriverNotInstalled) {
		return ErrDriverNotInstalled
	}
	return fmt.Errorf("%w: %v", ErrDLLLoadFailed, err)
}

func ensureHID() error {
	return translateHIDInit(callHIDInit())
}

func SetBackend(b Backend) error {
	backendMutex.Lock()
	defer backendMutex.Unlock()

	if b == BackendHID {
		// Eager initialization: Fail fast if driver/DLL is missing.
		// Any integer other than BackendHID is stored as-is and later follows
		// the message path. That includes values that NewInput rejects.
		if err := ensureHID(); err != nil {
			return err
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
		if err := ensureHID(); err != nil {
			return err
		}
	}
	return nil
}

// acquireBackend returns fixed when it is non-nil. That path does not read
// the global backend and does not initialize HID. A nil fixed uses the global
// backend, initializing HID when that backend is BackendHID.
func acquireBackend(fixed *Backend) (Backend, error) {
	if fixed != nil {
		return *fixed, nil
	}
	if err := checkBackend(); err != nil {
		return 0, err
	}
	return getBackend(), nil
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
			tx, ty, ok := addCursorDelta(cx, cy, x, y)
			if !ok {
				return errRelativeMoveOverflow
			}
			return hid.Move(tx, ty)
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
		tx, ty, ok := addCursorDelta(sx, sy, x, y)
		if !ok {
			return errRelativeMoveOverflow
		}
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
	return moveWindow(w, x, y, false, nil)
}

func moveWindow(w *Window, x, y int32, isRelative bool, fixed *Backend) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := w.checkReady(); err != nil {
		return err
	}
	cb, err := acquireBackend(fixed)
	if err != nil {
		return err
	}
	return moveImpl(cb, w.HWND, x, y, isRelative)
}

// MoveRel simulates relative mouse movement from the current cursor position.
// The cursor plus the delta must fit in int32. An overflowing sum returns an
// error and does not move.
func (w *Window) MoveRel(dx, dy int32) error {
	return moveWindow(w, dx, dy, true, nil)
}

// Click simulates a left mouse button click at the specified client coordinates.
func (w *Window) Click(x, y int32) error {
	return clickWindow(w, x, y, nil)
}

func clickWindow(w *Window, x, y int32, fixed *Backend) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := w.checkReady(); err != nil {
		return err
	}
	cb, err := acquireBackend(fixed)
	if err != nil {
		return err
	}

	if cb == BackendHID {
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
	return clickRightWindow(w, x, y, nil)
}

func clickRightWindow(w *Window, x, y int32, fixed *Backend) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := w.checkReady(); err != nil {
		return err
	}
	cb, err := acquireBackend(fixed)
	if err != nil {
		return err
	}

	if cb == BackendHID {
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
	return clickMiddleWindow(w, x, y, nil)
}

func clickMiddleWindow(w *Window, x, y int32, fixed *Backend) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := w.checkReady(); err != nil {
		return err
	}
	cb, err := acquireBackend(fixed)
	if err != nil {
		return err
	}

	if cb == BackendHID {
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
	return doubleClickWindow(w, x, y, nil)
}

func doubleClickWindow(w *Window, x, y int32, fixed *Backend) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := w.checkReady(); err != nil {
		return err
	}
	cb, err := acquireBackend(fixed)
	if err != nil {
		return err
	}

	if cb == BackendHID {
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
// delta == 0 is a no-op. Any other delta must fit in int16, or the call
// returns ErrInvalidScrollDelta (the mouse sentinel). Under BackendHID,
// delta is not validated and this method always returns
// ErrDirectedScrollUnsupported: a hardware wheel event cannot be addressed
// to a specific window. Use ScrollWheel for hardware-level scrolling.
func (w *Window) Scroll(x, y int32, delta int32) error {
	return scrollWindow(w, x, y, delta, nil)
}

func scrollWindow(w *Window, x, y, delta int32, fixed *Backend) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := w.checkReady(); err != nil {
		return err
	}
	cb, err := acquireBackend(fixed)
	if err != nil {
		return err
	}

	if cb == BackendHID {
		return ErrDirectedScrollUnsupported
	}
	return mouse.Scroll(w.HWND, x, y, delta)
}

// -----------------------------------------------------------------------------
// Global Input API (Screen Coordinates)
// -----------------------------------------------------------------------------

// MoveMouseTo moves the mouse cursor to the specified absolute screen coordinates (Virtual Desktop).
func MoveMouseTo(x, y int32) error {
	return moveMouseTo(x, y, nil)
}

func moveMouseTo(x, y int32, fixed *Backend) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	cb, err := acquireBackend(fixed)
	if err != nil {
		return err
	}

	if cb == BackendHID {
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
	return clickMouseAt(x, y, nil)
}

func clickMouseAt(x, y int32, fixed *Backend) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	cb, err := acquireBackend(fixed)
	if err != nil {
		return err
	}

	if cb == BackendHID {
		return hid.Click(x, y)
	}

	// Message Backend Fallback (duplicated logic from MoveMouseTo to avoid calling locked func)
	r, _, _ := window.ProcSetCursorPos.Call(uintptr(x), uintptr(y))
	if r == 0 {
		return fmt.Errorf("SetCursorPos failed")
	}

	time.Sleep(30 * time.Millisecond)
	if err := mouseButtonImpl(BackendMessage, MouseLeft, true); err != nil {
		return err
	}
	return mouseButtonImpl(BackendMessage, MouseLeft, false)
}

// DoubleClickMouseAt moves to the specified screen coordinates and performs a left double-click.
//
// Under BackendMessage the clicks use mouse_event, which has no return value;
// delivery failures of the button events are therefore not observable.
func DoubleClickMouseAt(x, y int32) error {
	return doubleClickMouseAt(x, y, nil)
}

func doubleClickMouseAt(x, y int32, fixed *Backend) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	cb, err := acquireBackend(fixed)
	if err != nil {
		return err
	}

	if cb == BackendHID {
		return hid.DoubleClick(x, y)
	}

	// Message Backend Fallback
	r, _, _ := window.ProcSetCursorPos.Call(uintptr(x), uintptr(y))
	if r == 0 {
		return fmt.Errorf("SetCursorPos failed")
	}

	// Double click = Down -> Up -> (delay) -> Down -> Up.
	// mouse_event has no DBLCLK flag, so this is two clicks.
	time.Sleep(30 * time.Millisecond)
	if err := mouseButtonImpl(BackendMessage, MouseLeft, true); err != nil {
		return err
	}
	if err := mouseButtonImpl(BackendMessage, MouseLeft, false); err != nil {
		return err
	}

	// Interval (short enough for OS to register as double click)
	time.Sleep(50 * time.Millisecond)

	if err := mouseButtonImpl(BackendMessage, MouseLeft, true); err != nil {
		return err
	}
	return mouseButtonImpl(BackendMessage, MouseLeft, false)
}

// ClickRightMouseAt moves to the specified screen coordinates and performs a right click.
//
// Under BackendMessage the click uses mouse_event, which has no return value;
// delivery failures of the button events are therefore not observable.
func ClickRightMouseAt(x, y int32) error {
	return clickRightMouseAt(x, y, nil)
}

func clickRightMouseAt(x, y int32, fixed *Backend) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	cb, err := acquireBackend(fixed)
	if err != nil {
		return err
	}

	if cb == BackendHID {
		return hid.ClickRight(x, y)
	}

	r, _, _ := window.ProcSetCursorPos.Call(uintptr(x), uintptr(y))
	if r == 0 {
		return fmt.Errorf("SetCursorPos failed")
	}

	time.Sleep(30 * time.Millisecond)
	if err := mouseButtonImpl(BackendMessage, MouseRight, true); err != nil {
		return err
	}
	return mouseButtonImpl(BackendMessage, MouseRight, false)
}

// ClickMiddleMouseAt moves to the specified screen coordinates and performs a middle click.
//
// Under BackendMessage the click uses mouse_event, which has no return value;
// delivery failures of the button events are therefore not observable.
func ClickMiddleMouseAt(x, y int32) error {
	return clickMiddleMouseAt(x, y, nil)
}

func clickMiddleMouseAt(x, y int32, fixed *Backend) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	cb, err := acquireBackend(fixed)
	if err != nil {
		return err
	}

	if cb == BackendHID {
		return hid.ClickMiddle(x, y)
	}

	r, _, _ := window.ProcSetCursorPos.Call(uintptr(x), uintptr(y))
	if r == 0 {
		return fmt.Errorf("SetCursorPos failed")
	}

	time.Sleep(30 * time.Millisecond)
	if err := mouseButtonImpl(BackendMessage, MouseMiddle, true); err != nil {
		return err
	}
	return mouseButtonImpl(BackendMessage, MouseMiddle, false)
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
	return mouseDown(button, nil)
}

func mouseDown(button MouseButton, fixed *Backend) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	cb, err := acquireBackend(fixed)
	if err != nil {
		return err
	}
	return mouseButtonImpl(cb, button, true)
}

// MouseUp releases the specified button at the current cursor position.
// The input mutex covers this call only; a down/move/up sequence across
// separate calls is not atomic.
func MouseUp(button MouseButton) error {
	return mouseUp(button, nil)
}

func mouseUp(button MouseButton, fixed *Backend) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	cb, err := acquireBackend(fixed)
	if err != nil {
		return err
	}
	return mouseButtonImpl(cb, button, false)
}

// ClickMouse performs a left click at the current cursor position without moving.
//
// Button flags come from the same mapping as MouseDown and MouseUp.
// Under BackendMessage the click uses mouse_event, which has no return value;
// delivery failures of the button events are therefore not observable.
// Under BackendHID a failed release is retried once, and both failures keep
// the first error.
func ClickMouse() error {
	return clickMouse(nil)
}

func clickMouse(fixed *Backend) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	cb, err := acquireBackend(fixed)
	if err != nil {
		return err
	}
	if err := mouseButtonImpl(cb, MouseLeft, true); err != nil {
		return err
	}
	time.Sleep(50 * time.Millisecond)
	if err := mouseButtonImpl(cb, MouseLeft, false); err != nil {
		// mouse_event cannot report failure. Only the HID release is
		// retried, and both failures keep the first error.
		if cb == BackendHID {
			if err2 := mouseButtonImpl(cb, MouseLeft, false); err2 != nil {
				return err
			}
			return nil
		}
		return err
	}
	return nil
}

// errRelativeMoveOverflow is returned when a cursor plus a delta does not fit
// in int32. MoveMouseBy and Window.MoveRel share it.
var errRelativeMoveOverflow = errors.New("relative move result exceeds int32")

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
	return moveMouseBy(dx, dy, nil)
}

func moveMouseBy(dx, dy int32, fixed *Backend) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	cb, err := acquireBackend(fixed)
	if err != nil {
		return err
	}

	if cb == BackendHID {
		return hid.MoveRelative(dx, dy)
	}

	cx, cy, err := window.GetCursorPos()
	if err != nil {
		return err
	}
	nx, ny, ok := addCursorDelta(cx, cy, dx, dy)
	if !ok {
		return errRelativeMoveOverflow
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
	return moveMouseToExact(x, y, nil)
}

func moveMouseToExact(x, y int32, fixed *Backend) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	cb, err := acquireBackend(fixed)
	if err != nil {
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

	if cb == BackendHID {
		return hid.MoveExact(x, y)
	}

	r, _, _ := window.ProcSetCursorPos.Call(uintptr(x), uintptr(y))
	if r == 0 {
		return fmt.Errorf("SetCursorPos failed")
	}
	return nil
}

// mouseEventWheel is MOUSEEVENTF_WHEEL.
const mouseEventWheel = 0x0800

// mouseEventWheelArgs returns the mouse_event arguments for one wheel
// stroke. The signature is (dwFlags, dx, dy, dwData, dwExtraInfo).
func mouseEventWheelArgs(delta int32) (flags, dx, dy, data, extra uintptr) {
	return mouseEventWheel, 0, 0, uintptr(uint32(delta)), 0
}

// ScrollWheel scrolls by whole wheel steps. Positive steps scroll forward.
// Zero and values outside [-273, 273] are rejected before multiplication.
func ScrollWheel(steps int) error {
	return scrollWheel(steps, nil)
}

func scrollWheel(steps int, fixed *Backend) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	cb, err := acquireBackend(fixed)
	if err != nil {
		return err
	}
	if steps == 0 || steps < -273 || steps > 273 {
		return ErrInvalidScrollSteps
	}

	delta := int32(steps * mouse.WHEEL_DELTA)
	if cb == BackendHID {
		return hid.Scroll(delta)
	}

	// mouse_event(MOUSEEVENTF_WHEEL, 0, 0, delta, 0)
	flags, dx, dy, data, extra := mouseEventWheelArgs(delta)
	window.ProcMouseEvent.Call(flags, dx, dy, data, extra)
	return nil
}

// PressFor holds a global key for at least 10ms.
func PressFor(key Key, duration time.Duration) (err error) {
	return pressFor(key, duration, nil)
}

func pressFor(key Key, duration time.Duration, fixed *Backend) (err error) {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	cb, err := acquireBackend(fixed)
	if err != nil {
		return err
	}
	if duration < 10*time.Millisecond {
		return ErrInvalidPressDuration
	}
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

// defaultPressHold is the hold time for Press when the caller supplies no
// interval: the time between the successful key-down and the release. It is a
// separate concept from keyboard.DefaultTypeGap (the Type character gap);
// changing one must not change the other.
const defaultPressHold = 30 * time.Millisecond

// resolveInterval parses the optional interval variadic. No argument yields
// fallback with explicit == false. Exactly one non-negative argument yields
// that value with explicit == true (0 means "no sleep at this point"). A
// negative value or more than one argument returns ErrInvalidInterval. The
// returned error is meant to fail the caller before it locks inputMutex or
// presses anything.
func resolveInterval(interval []time.Duration, fallback time.Duration) (d time.Duration, explicit bool, err error) {
	switch len(interval) {
	case 0:
		return fallback, false, nil
	case 1:
		if interval[0] < 0 {
			return 0, false, ErrInvalidInterval
		}
		return interval[0], true, nil
	default:
		return 0, false, ErrInvalidInterval
	}
}

// Public Wrappers using Lock

// KeyDown sends a key down event to the window.
func (w *Window) KeyDown(key Key) error {
	return keyDownWindow(w, key, nil)
}

func keyDownWindow(w *Window, key Key, fixed *Backend) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := w.checkReady(); err != nil {
		return err
	}
	cb, err := acquireBackend(fixed)
	if err != nil {
		return err
	}
	return keyDownImpl(cb, w.HWND, key)
}

// KeyUp sends a key up event to the window.
func (w *Window) KeyUp(key Key) error {
	return keyUpWindow(w, key, nil)
}

func keyUpWindow(w *Window, key Key, fixed *Backend) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := w.checkReady(); err != nil {
		return err
	}
	cb, err := acquireBackend(fixed)
	if err != nil {
		return err
	}
	return keyUpImpl(cb, w.HWND, key)
}

// Press simulates a key press (down then up).
//
// The optional interval is the hold time: the delay between a successful
// key-down and the release. It is not a Type character gap and not a "typing
// speed" knob. The two backends both honor it. Without an argument the hold is
// defaultPressHold; an explicit 0 means no sleep between down and up. A
// negative interval or more than one interval returns ErrInvalidInterval
// before any key is pressed.
func (w *Window) Press(key Key, interval ...time.Duration) (err error) {
	hold, _, err := resolveInterval(interval, defaultPressHold)
	if err != nil {
		return err
	}

	return pressWindow(w, key, hold, nil)
}

func pressWindow(w *Window, key Key, hold time.Duration, fixed *Backend) (err error) {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := w.checkReady(); err != nil {
		return err
	}
	cb, err := acquireBackend(fixed)
	if err != nil {
		return err
	}

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
	if hold > 0 {
		time.Sleep(hold)
	}
	return nil
}

// PressHotkey presses a combination of keys (e.g., Ctrl+A).
// Under BackendMessage, modifier combinations (Ctrl/Alt/Shift + another key)
// are promoted to foreground keyboard events because PostMessage alone does
// not provide reliable modifier state to many target windows.
func (w *Window) PressHotkey(keys ...Key) (err error) {
	return pressHotkeyWindow(w, nil, keys...)
}

func pressHotkeyWindow(w *Window, fixed *Backend, keys ...Key) (err error) {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := w.checkReady(); err != nil {
		return err
	}
	cb, err := acquireBackend(fixed)
	if err != nil {
		return err
	}
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
//
// The optional interval is the character gap: on both backends it is the idle
// time between characters, not a whole typing rhythm and not a Press hold.
// An explicit 30ms is not the same as no argument on HID: the message backend
// sleeps the gap between runes in both cases, while HID keeps its fixed hold
// and sleeps the gap between character releases only when the caller supplied
// an interval. The last rune is not followed by an extra sleep, and consecutive
// Type calls do not sleep between each other. A negative interval or more than
// one interval returns ErrInvalidInterval before any character is sent.
func (w *Window) Type(text string, interval ...time.Duration) error {
	gap, explicit, err := resolveInterval(interval, keyboard.DefaultTypeGap)
	if err != nil {
		return err
	}

	return typeWindow(w, text, gap, explicit, nil)
}

func typeWindow(w *Window, text string, gap time.Duration, explicit bool, fixed *Backend) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := w.checkReady(); err != nil {
		return err
	}
	cb, err := acquireBackend(fixed)
	if err != nil {
		return err
	}

	if cb == BackendMessage {
		// Use WM_CHAR for reliability in background; the only sleep on this
		// path lives inside TypeWithInterval.
		return keyboard.TypeWithInterval(w.HWND, text, gap)
	}

	return typeHID(text, gap, explicit)
}

// typeHID types text with the HID backend. The whole string is validated
// before the first key is pressed; each character's Shift state is released
// before the next character starts.
//
// gap is the idle time between the releases of consecutive characters and is
// applied only when explicit is true: an unspecified interval adds no gap
// (Type(text) keeps today's rhythm of defaultPressHold per character with no
// idle time between), while Type(text, d) sleeps d after each character's
// Release returns. gap == 0 therefore only suppresses the extra gap; it never
// removes the fixed hold before Release.
func typeHID(text string, gap time.Duration, explicit bool) error {
	for _, r := range text {
		if _, _, ok := keyboard.LookupKey(r); !ok {
			return ErrUnsupportedKey
		}
	}

	runes := []rune(text)
	for i, r := range runes {
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
			time.Sleep(defaultPressHold)
			return nil
		}()
		if err != nil {
			return err
		}
		if explicit && gap > 0 && i < len(runes)-1 {
			time.Sleep(gap)
		}
	}
	return nil
}

// Global Wrappers

// KeyDown simulates a global key down event.
func KeyDown(k Key) error {
	return keyDownGlobal(k, nil)
}

func keyDownGlobal(k Key, fixed *Backend) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	cb, err := acquireBackend(fixed)
	if err != nil {
		return err
	}
	return keyDownImpl(cb, 0, k)
}

// KeyUp simulates a global key up event.
func KeyUp(k Key) error {
	return keyUpGlobal(k, nil)
}

func keyUpGlobal(k Key, fixed *Backend) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	cb, err := acquireBackend(fixed)
	if err != nil {
		return err
	}
	return keyUpImpl(cb, 0, k)
}

// Press simulates a global key press (down then up).
//
// The optional interval is the hold time (delay between a successful key-down
// and the release) and is honored on both backends. An explicit 0 means no
// sleep between down and up. A negative interval or more than one interval
// returns ErrInvalidInterval before any key is pressed.
func Press(k Key, interval ...time.Duration) (err error) {
	hold, _, err := resolveInterval(interval, defaultPressHold)
	if err != nil {
		return err
	}

	return pressGlobal(k, hold, nil)
}

func pressGlobal(k Key, hold time.Duration, fixed *Backend) (err error) {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	cb, err := acquireBackend(fixed)
	if err != nil {
		return err
	}

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
	if hold > 0 {
		time.Sleep(hold)
	}
	return nil
}

// PressHotkey simulates a global combination of keys.
func PressHotkey(keys ...Key) (err error) {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	return pressHotkeyLocked(keys...)
}

// pressHotkeyLocked is the body of global PressHotkey run while inputMutex is
// held. It must never call a public input function that would lock inputMutex
// again, including Press, PressHotkey, Type and PressFor. PasteByClipboard
// uses it so the clipboard swap and the Ctrl+V stay inside the same lock.
func pressHotkeyLocked(keys ...Key) (err error) {
	if err := checkBackend(); err != nil {
		return err
	}
	return pressHotkeyWith(getBackend(), keys...)
}

func pressHotkeyWith(cb Backend, keys ...Key) (err error) {
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
//
// The optional interval is the character gap on both backends (a Press hold
// on HID is fixed and does not read this argument). An explicit 30ms is not
// the same as no argument on HID: the message backend sleeps the gap between
// runes in both cases, while HID keeps its fixed hold and sleeps the gap
// between character releases only when the caller supplied an interval. The
// last rune is not followed by an extra sleep. A negative interval or more
// than one interval returns ErrInvalidInterval before any character is sent.
func Type(text string, interval ...time.Duration) error {
	gap, explicit, err := resolveInterval(interval, keyboard.DefaultTypeGap)
	if err != nil {
		return err
	}

	return typeGlobal(text, gap, explicit, nil)
}

func typeGlobal(text string, gap time.Duration, explicit bool, fixed *Backend) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	cb, err := acquireBackend(fixed)
	if err != nil {
		return err
	}

	if cb == BackendHID {
		return typeHID(text, gap, explicit)
	}

	runes := []rune(text)
	for i, r := range runes {
		if err := sendUnicode(r); err != nil {
			return err
		}
		if i < len(runes)-1 && gap > 0 {
			time.Sleep(gap)
		}
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

// ClientInfo returns the client-area rectangle as screen coordinates.
// It returns the screen coordinate of the client-area origin (0, 0) plus the
// client width and height. The rectangle excludes the title bar and borders.
//
// The order is fixed: first ClientToScreen(0, 0), then ClientRect. If either
// call fails, all four integers are zero and the error is returned; a failed
// first call does not reach the second one. A minimized window follows the
// existing "window is minimized" error from window.ClientToScreen.
//
// DPI: the coordinates use the DPI awareness the calling thread already has.
// Aligning ClientInfo with screen.CaptureRegion requires the process to have
// the same awareness as the screenshot; this method does not call
// EnablePerMonitorDPI and does not convert physical pixels.
//
// The four values are read by two independent Win32 calls and are not an
// atomic snapshot: another process can move or resize the window between them.
func (w *Window) ClientInfo() (x, y, width, height int32, err error) {
	x, y, err = w.ClientToScreen(0, 0)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	width, height, err = w.ClientRect()
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return x, y, width, height, nil
}

// ScreenToClient converts screen coordinates to client coordinates.
func (w *Window) ScreenToClient(x, y int32) (cx, cy int32, err error) {
	return window.ScreenToClient(w.HWND, x, y)
}

// ClientToScreen converts client coordinates to screen coordinates.
func (w *Window) ClientToScreen(x, y int32) (sx, sy int32, err error) {
	return window.ClientToScreen(w.HWND, x, y)
}
