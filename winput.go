package winput

import (
	"errors"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/rpdg/winput/hid"
	"github.com/rpdg/winput/keyboard"
	"github.com/rpdg/winput/mouse"
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
func SetBackend(b Backend) {
	backendMutex.Lock()
	defer backendMutex.Unlock()
	currentBackend = b
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
		vk := keyboard.MapScanCodeToVK(k)
		window.ProcKeybdEvent.Call(vk, 0, 0, 0)
		return nil
	}
	return keyboard.KeyDown(hwnd, k)
}

func keyUpImpl(cb Backend, hwnd uintptr, k Key) error {
	if cb == BackendHID {
		return hid.KeyUp(uint16(k))
	}
	if hwnd == 0 {
		vk := keyboard.MapScanCodeToVK(k)
		window.ProcKeybdEvent.Call(vk, 0, 0x0002, 0)
		return nil
	}
	return keyboard.KeyUp(hwnd, k)
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

// Scroll simulates a vertical mouse wheel scroll.
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
		return hid.Scroll(delta)
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

// -----------------------------------------------------------------------------
// Input API (Keyboard)
// -----------------------------------------------------------------------------

type Key = keyboard.Key

const (
	KeyEsc       = keyboard.KeyEsc
	Key1         = keyboard.Key1
	Key2         = keyboard.Key2
	Key3         = keyboard.Key3
	Key4         = keyboard.Key4
	Key5         = keyboard.Key5
	Key6         = keyboard.Key6
	Key7         = keyboard.Key7
	Key8         = keyboard.Key8
	Key9         = keyboard.Key9
	Key0         = keyboard.Key0
	KeyMinus     = keyboard.KeyMinus
	KeyEqual     = keyboard.KeyEqual
	KeyBkSp      = keyboard.KeyBkSp
	KeyTab       = keyboard.KeyTab
	KeyQ         = keyboard.KeyQ
	KeyW         = keyboard.KeyW
	KeyE         = keyboard.KeyE
	KeyR         = keyboard.KeyR
	KeyT         = keyboard.KeyT
	KeyY         = keyboard.KeyY
	KeyU         = keyboard.KeyU
	KeyI         = keyboard.KeyI
	KeyO         = keyboard.KeyO
	KeyP         = keyboard.KeyP
	KeyLBr       = keyboard.KeyLBr
	KeyRBr       = keyboard.KeyRBr
	KeyEnter     = keyboard.KeyEnter
	KeyCtrl      = keyboard.KeyCtrl
	KeyA         = keyboard.KeyA
	KeyS         = keyboard.KeyS
	KeyD         = keyboard.KeyD
	KeyF         = keyboard.KeyF
	KeyG         = keyboard.KeyG
	KeyH         = keyboard.KeyH
	KeyJ         = keyboard.KeyJ
	KeyK         = keyboard.KeyK
	KeyL         = keyboard.KeyL
	KeySemi      = keyboard.KeySemi
	KeyQuot      = keyboard.KeyQuot
	KeyTick      = keyboard.KeyTick
	KeyShift     = keyboard.KeyShift
	KeyBackslash = keyboard.KeyBackslash
	KeyZ         = keyboard.KeyZ
	KeyX         = keyboard.KeyX
	KeyC         = keyboard.KeyC
	KeyV         = keyboard.KeyV
	KeyB         = keyboard.KeyB
	KeyN         = keyboard.KeyN
	KeyM         = keyboard.KeyM
	KeyComma     = keyboard.KeyComma
	KeyDot       = keyboard.KeyDot
	KeySlash     = keyboard.KeySlash
	KeyAlt       = keyboard.KeyAlt
	KeySpace     = keyboard.KeySpace
	KeyCaps      = keyboard.KeyCaps
	KeyF1        = keyboard.KeyF1
	KeyF2        = keyboard.KeyF2
	KeyF3        = keyboard.KeyF3
	KeyF4        = keyboard.KeyF4
	KeyF5        = keyboard.KeyF5
	KeyF6        = keyboard.KeyF6
	KeyF7        = keyboard.KeyF7
	KeyF8        = keyboard.KeyF8
	KeyF9        = keyboard.KeyF9
	KeyF10       = keyboard.KeyF10
	KeyF11       = keyboard.KeyF11
	KeyF12       = keyboard.KeyF12
	KeyNumLock   = keyboard.KeyNumLock
	KeyScroll    = keyboard.KeyScroll

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
func (w *Window) Press(key Key) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := w.checkReady(); err != nil {
		return err
	}
	if err := checkBackend(); err != nil {
		return err
	}

	if err := keyDownImpl(getBackend(), w.HWND, key); err != nil {
		return err
	}
	time.Sleep(30 * time.Millisecond)
	return keyUpImpl(getBackend(), w.HWND, key)
}

// PressHotkey presses a combination of keys (e.g., Ctrl+A).
func (w *Window) PressHotkey(keys ...Key) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := w.checkReady(); err != nil {
		return err
	}
	if err := checkBackend(); err != nil {
		return err
	}

	cb := getBackend()
	for _, k := range keys {
		if err := keyDownImpl(cb, w.HWND, k); err != nil {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(30 * time.Millisecond)
	for i := len(keys) - 1; i >= 0; i-- {
		if err := keyUpImpl(cb, w.HWND, keys[i]); err != nil {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
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

	// HID Backend simulation
	for _, r := range text {
		k, shifted, ok := keyboard.LookupKey(r)
		if !ok {
			return ErrUnsupportedKey
		}

		if shifted {
			hid.KeyDown(uint16(KeyShift))
			time.Sleep(10 * time.Millisecond)
			hid.Press(uint16(k))
			hid.KeyUp(uint16(KeyShift))
		} else {
			hid.Press(uint16(k))
		}
		time.Sleep(30 * time.Millisecond)
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
func Press(k Key) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := checkBackend(); err != nil {
		return err
	}

	if err := keyDownImpl(getBackend(), 0, k); err != nil {
		return err
	}
	time.Sleep(30 * time.Millisecond)
	return keyUpImpl(getBackend(), 0, k)
}

// PressHotkey simulates a global combination of keys.
func PressHotkey(keys ...Key) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := checkBackend(); err != nil {
		return err
	}

	cb := getBackend()
	for _, k := range keys {
		if err := keyDownImpl(cb, 0, k); err != nil {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(30 * time.Millisecond)
	for i := len(keys) - 1; i >= 0; i-- {
		if err := keyUpImpl(cb, 0, keys[i]); err != nil {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

var (
	sendInputOnce sync.Once
	sendInputErr  error
)

// Type simulates typing text globally.
func Type(text string) error {
	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := checkBackend(); err != nil {
		return err
	}

	cb := getBackend()
	if cb == BackendHID {
		for _, r := range text {
			k, shifted, ok := keyboard.LookupKey(r)
			if !ok {
				return ErrUnsupportedKey
			}
			if shifted {
				hid.KeyDown(uint16(KeyShift))
				time.Sleep(10 * time.Millisecond)
				hid.Press(uint16(k))
				hid.KeyUp(uint16(KeyShift))
			} else {
				hid.Press(uint16(k))
			}
			time.Sleep(30 * time.Millisecond)
		}
		return nil
	}

	// Message Backend Fallback: SendInput with Unicode
	sendInputOnce.Do(func() {
		// Self-test to check if SendInput is viable (permissions, etc.)
		var inputs [1]input
		inputs[0].Type = INPUT_KEYBOARD
		inputs[0].Ki.WScan = 'A' // Dummy char
		inputs[0].Ki.DwFlags = KEYEVENTF_UNICODE

		n, _, _ := window.ProcSendInput.Call(1, uintptr(unsafe.Pointer(&inputs[0])), uintptr(unsafe.Sizeof(inputs[0])))
		if n == 0 {
			sendInputErr = errors.New("SendInput self-test failed; unsupported in this context")
		}
	})
	if sendInputErr != nil {
		return sendInputErr
	}

	for _, r := range text {
		sendUnicode(r)
		time.Sleep(30 * time.Millisecond)
	}
	return nil
}

// Internal structures for SendInput
type keyboardInput struct {
	WVk     uint16
	WScan   uint16
	DwFlags uint32
	Time    uint32
	DwExtra uintptr
}
type input struct {
	Type uint32
	Ki   keyboardInput
}

const (
	INPUT_KEYBOARD    = 1
	KEYEVENTF_UNICODE = 0x0004
	KEYEVENTF_KEYUP   = 0x0002
)

func sendUnicode(r rune) {
	var inputs [2]input
	inputs[0].Type = INPUT_KEYBOARD
	inputs[0].Ki.WScan = uint16(r)
	inputs[0].Ki.DwFlags = KEYEVENTF_UNICODE

	inputs[1] = inputs[0]
	inputs[1].Ki.DwFlags = KEYEVENTF_UNICODE | KEYEVENTF_KEYUP

	window.ProcSendInput.Call(2, uintptr(unsafe.Pointer(&inputs[0])), uintptr(unsafe.Sizeof(inputs[0])))
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
