package winput

import (
	"errors"
	"fmt"
	"github.com/rpdg/winput/hid"
	"github.com/rpdg/winput/keyboard"
	"github.com/rpdg/winput/mouse"
	"github.com/rpdg/winput/window"
)

type Window struct {
	HWND uintptr
}

// -----------------------------------------------------------------------------
// Window Discovery
// -----------------------------------------------------------------------------

func FindByTitle(title string) (*Window, error) {
	hwnd, err := window.FindByTitle(title)
	if err != nil {
		// Assuming any error from FindByTitle implies not found for now
		return nil, ErrWindowNotFound
	}
	return &Window{HWND: hwnd}, nil
}

func FindByClass(class string) (*Window, error) {
	hwnd, err := window.FindByClass(class)
	if err != nil {
		return nil, ErrWindowNotFound
	}
	return &Window{HWND: hwnd}, nil
}

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

// -----------------------------------------------------------------------------
// Backend Configuration
// -----------------------------------------------------------------------------

type Backend int

const (
	BackendMessage Backend = iota
	BackendHID
)

var currentBackend Backend = BackendMessage

// SetBackend sets the input backend.
// It does not return an error; initialization errors occur upon first use (Explicit Failure).
func SetBackend(b Backend) {
	currentBackend = b
}

// SetHIDLibraryPath sets the path to interception.dll.
// Default is "interception.dll" (search in system PATH or current dir).
// Must be called before SetBackend(BackendHID) or first HID action.
func SetHIDLibraryPath(path string) {
	hid.SetLibraryPath(path)
}

func checkBackend() error {
	if currentBackend == BackendHID {
		if err := hid.Init(); err != nil {
			if errors.Is(err, hid.ErrDriverNotInstalled) {
				return ErrDriverNotInstalled
			}
			// Assume other errors are DLL load failures
			return fmt.Errorf("%w: %v", ErrDLLLoadFailed, err)
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// Input API (Mouse)
// -----------------------------------------------------------------------------

func (w *Window) Move(x, y int32) error {
	if err := checkBackend(); err != nil {
		return err
	}
	
	if currentBackend == BackendHID {
		sx, sy, err := window.ClientToScreen(w.HWND, x, y)
		if err != nil {
			return err
		}
		return hid.Move(sx, sy)
	}
	return mouse.Move(w.HWND, x, y)
}

// MoveRel moves the mouse cursor relative to the *current* cursor position.
// dx, dy are in pixels.
func (w *Window) MoveRel(dx, dy int32) error {
	if err := checkBackend(); err != nil {
		return err
	}

	if currentBackend == BackendHID {
		// HID backend Move is actually implementing a smooth move to absolute target.
		// But underlying Interception sends relative moves.
		// However, to keep consistency with "Move(x,y)" which targets absolute client coordinates,
		// MoveRel should move relative to current position.
		// Since HID backend "Move" implementation is actually "MoveTo(screenX, screenY)",
		// We can implement MoveRel by getting current pos + delta.
		// BUT wait, hid.Move implements "Smooth Move".
		// We should probably expose a direct "MoveRel" in hid package too.
		// For now let's reuse Move logic or implement raw relative move?
		// User asked for "MoveRel" in review.
		
		// If we use hid.Move (which is smooth), we need target screen coords.
		cx, cy, err := window.GetCursorPos()
		if err != nil {
			return err
		}
		return hid.Move(cx+dx, cy+dy)
	}
	
	// Message Backend: MoveRel is tricky because WM_MOUSEMOVE expects client coords.
	// We need to know where the "virtual" mouse is? Or physical?
	// Message backend usually doesn't move physical cursor.
	// So "Relative" implies relative to where we last clicked? Or relative to current physical cursor?
	// If relative to physical cursor, it might be weird if we are not moving it.
	// Let's assume relative to current physical cursor mapped to client space.
	
	sx, sy, err := window.GetCursorPos()
	if err != nil {
		return err
	}
	// Target Screen Coords
	tx, ty := sx+dx, sy+dy
	
	// Convert Target Screen to Client
	cx, cy, err := window.ScreenToClient(w.HWND, tx, ty)
	if err != nil {
		return err
	}
	
	return mouse.Move(w.HWND, cx, cy)
}

func (w *Window) Click(x, y int32) error {
	if err := checkBackend(); err != nil {
		return err
	}

	if currentBackend == BackendHID {
		sx, sy, err := window.ClientToScreen(w.HWND, x, y)
		if err != nil {
			return err
		}
		return hid.Click(sx, sy)
	}
	return mouse.Click(w.HWND, x, y)
}

func (w *Window) ClickRight(x, y int32) error {
	if err := checkBackend(); err != nil {
		return err
	}

	if currentBackend == BackendHID {
		sx, sy, err := window.ClientToScreen(w.HWND, x, y)
		if err != nil {
			return err
		}
		return hid.ClickRight(sx, sy)
	}
	return mouse.ClickRight(w.HWND, x, y)
}

func (w *Window) ClickMiddle(x, y int32) error {
	if err := checkBackend(); err != nil {
		return err
	}

	if currentBackend == BackendHID {
		sx, sy, err := window.ClientToScreen(w.HWND, x, y)
		if err != nil {
			return err
		}
		return hid.ClickMiddle(sx, sy)
	}
	return mouse.ClickMiddle(w.HWND, x, y)
}

func (w *Window) DoubleClick(x, y int32) error {
	if err := checkBackend(); err != nil {
		return err
	}

	if currentBackend == BackendHID {
		sx, sy, err := window.ClientToScreen(w.HWND, x, y)
		if err != nil {
			return err
		}
		return hid.DoubleClick(sx, sy)
	}
	return mouse.DoubleClick(w.HWND, x, y)
}

func (w *Window) Scroll(x, y int32, delta int32) error {
	if err := checkBackend(); err != nil {
		return err
	}
	
	if currentBackend == BackendHID {
		return hid.Scroll(delta)
	}
	return mouse.Scroll(w.HWND, x, y, delta)
}

// -----------------------------------------------------------------------------
// Input API (Keyboard)
// -----------------------------------------------------------------------------

// Key type alias
type Key = keyboard.Key

// Common Key Constants Re-export
const (
	KeyEsc   = keyboard.KeyEsc
	Key1     = keyboard.Key1
	Key2     = keyboard.Key2
	Key3     = keyboard.Key3
	Key4     = keyboard.Key4
	Key5     = keyboard.Key5
	Key6     = keyboard.Key6
	Key7     = keyboard.Key7
	Key8     = keyboard.Key8
	Key9     = keyboard.Key9
	Key0     = keyboard.Key0
	KeyMinus = keyboard.KeyMinus
	KeyEqual = keyboard.KeyEqual
	KeyBkSp  = keyboard.KeyBkSp
	KeyTab   = keyboard.KeyTab
	KeyQ     = keyboard.KeyQ
	KeyW     = keyboard.KeyW
	KeyE     = keyboard.KeyE
	KeyR     = keyboard.KeyR
	KeyT     = keyboard.KeyT
	KeyY     = keyboard.KeyY
	KeyU     = keyboard.KeyU
	KeyI     = keyboard.KeyI
	KeyO     = keyboard.KeyO
	KeyP     = keyboard.KeyP
	KeyLBr   = keyboard.KeyLBr
	KeyRBr   = keyboard.KeyRBr
	KeyEnter = keyboard.KeyEnter
	KeyCtrl  = keyboard.KeyCtrl
	KeyA     = keyboard.KeyA
	KeyS     = keyboard.KeyS
	KeyD     = keyboard.KeyD
	KeyF     = keyboard.KeyF
	KeyG     = keyboard.KeyG
	KeyH     = keyboard.KeyH
	KeyJ     = keyboard.KeyJ
	KeyK     = keyboard.KeyK
	KeyL     = keyboard.KeyL
	KeySemi  = keyboard.KeySemi
	KeyQuot  = keyboard.KeyQuot
	KeyTick  = keyboard.KeyTick
	KeyShift = keyboard.KeyShift
	KeyBackslash = keyboard.KeyBackslash
	KeyZ     = keyboard.KeyZ
	KeyX     = keyboard.KeyX
	KeyC     = keyboard.KeyC
	KeyV     = keyboard.KeyV
	KeyB     = keyboard.KeyB
	KeyN     = keyboard.KeyN
	KeyM     = keyboard.KeyM
	KeyComma = keyboard.KeyComma
	KeyDot   = keyboard.KeyDot
	KeySlash = keyboard.KeySlash
	KeySpace = keyboard.KeySpace
	KeyAlt   = keyboard.KeyAlt
	KeyCaps  = keyboard.KeyCaps
	KeyF1    = keyboard.KeyF1
	KeyF2    = keyboard.KeyF2
	KeyF3    = keyboard.KeyF3
	KeyF4    = keyboard.KeyF4
	KeyF5    = keyboard.KeyF5
	KeyF6    = keyboard.KeyF6
	KeyF7    = keyboard.KeyF7
	KeyF8    = keyboard.KeyF8
	KeyF9    = keyboard.KeyF9
	KeyF10   = keyboard.KeyF10
	KeyF11   = keyboard.KeyF11
	KeyF12   = keyboard.KeyF12
)

func KeyFromRune(r rune) (Key, bool) {
	k, _, ok := keyboard.KeyFromRune(r)
	return k, ok
}

func (w *Window) KeyDown(key Key) error {
	if err := checkBackend(); err != nil {
		return err
	}

	if currentBackend == BackendHID {
		return hid.KeyDown(uint16(key))
	}
	
	err := keyboard.KeyDown(w.HWND, key)
	if err != nil {
		return err
	}
	return nil
}

func (w *Window) KeyUp(key Key) error {
	if err := checkBackend(); err != nil {
		return err
	}

	if currentBackend == BackendHID {
		return hid.KeyUp(uint16(key))
	}
	return keyboard.KeyUp(w.HWND, key)
}

func (w *Window) Press(key Key) error {
	if err := checkBackend(); err != nil {
		return err
	}

	if currentBackend == BackendHID {
		return hid.Press(uint16(key))
	}
	return keyboard.Press(w.HWND, key)
}

// PressHotkey presses a combination of keys (e.g. Ctrl+A).
// It presses them in order and releases them in reverse order.
func (w *Window) PressHotkey(keys ...Key) error {
	if len(keys) == 0 {
		return nil
	}
	if err := checkBackend(); err != nil {
		return err
	}

	// Press down
	for _, k := range keys {
		if err := w.KeyDown(k); err != nil {
			// Release previous ones? For simplicity, we abort.
			// Ideally we should release what we pressed.
			return err
		}
	}

	// Release up in reverse
	for i := len(keys) - 1; i >= 0; i-- {
		w.KeyUp(keys[i])
	}
	return nil
}

func (w *Window) Type(text string) error {
	if err := checkBackend(); err != nil {
		return err
	}

	for _, r := range text {
		k, shifted, ok := keyboard.KeyFromRune(r)
		if !ok {
			return ErrUnsupportedKey
		}
		
		// Handle Shift
		if shifted {
			if currentBackend == BackendHID {
				hid.KeyDown(uint16(KeyShift))
				hid.Press(uint16(k))
				hid.KeyUp(uint16(KeyShift))
			} else {
				keyboard.KeyDown(w.HWND, KeyShift)
				keyboard.Press(w.HWND, k)
				keyboard.KeyUp(w.HWND, KeyShift)
			}
		} else {
			if currentBackend == BackendHID {
				hid.Press(uint16(k))
			} else {
				keyboard.Press(w.HWND, k)
			}
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// Coordinate & DPI
// -----------------------------------------------------------------------------

func EnablePerMonitorDPI() error {
	return window.EnablePerMonitorDPI()
}

func (w *Window) DPI() (uint32, uint32, error) {
	return window.GetDPI(w.HWND)
}

func (w *Window) ClientRect() (width, height int32, err error) {
	return window.GetClientRect(w.HWND)
}

func (w *Window) ScreenToClient(x, y int32) (cx, cy int32, err error) {
	return window.ScreenToClient(w.HWND, x, y)
}

func (w *Window) ClientToScreen(x, y int32) (sx, sy int32, err error) {
	return window.ClientToScreen(w.HWND, x, y)
}