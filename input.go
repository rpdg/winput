package winput

import (
	"time"

	"github.com/rpdg/winput/keyboard"
)

// Input is an input executor bound to one backend chosen at construction.
// The zero value is not an executor. A later SetBackend does not change it.
type Input struct {
	backend Backend
	valid   bool
}

// WindowInput is the window-targeted facade of an Input.
// A zero value, or a value that did not come from Input.ForWindow, is invalid.
type WindowInput struct {
	input  *Input
	window *Window
}

// DesktopInput is the screen-coordinate and windowless facade of an Input.
// A zero value, or a value that did not come from Input.Desktop, is invalid.
type DesktopInput struct {
	input *Input
}

// NewInput builds an executor for backend. BackendMessage does not initialize
// HID. Any other integer, including negatives, returns ErrInvalidBackend
// before HID initialization. HID initialization failure returns the existing
// driver or DLL error and no instance.
func NewInput(backend Backend) (*Input, error) {
	if backend != BackendMessage && backend != BackendHID {
		return nil, ErrInvalidBackend
	}
	if backend == BackendHID {
		if err := ensureHID(); err != nil {
			return nil, err
		}
	}
	return &Input{backend: backend, valid: true}, nil
}

// ForWindow binds w to this executor. A nil window is accepted here; calls on
// the result return ErrWindowGone. A zero or nil Input returns ErrInputInvalid
// and a nil facade.
func (i *Input) ForWindow(w *Window) (*WindowInput, error) {
	if i == nil || !i.valid {
		return nil, ErrInputInvalid
	}
	return &WindowInput{input: i, window: w}, nil
}

// Desktop returns the screen-coordinate facade of this executor.
func (i *Input) Desktop() (*DesktopInput, error) {
	if i == nil || !i.valid {
		return nil, ErrInputInvalid
	}
	return &DesktopInput{input: i}, nil
}

func (in *WindowInput) ready() error {
	if in == nil || in.input == nil || !in.input.valid {
		return ErrInputInvalid
	}
	if in.window == nil {
		return ErrWindowGone
	}
	return nil
}

func (d *DesktopInput) ready() error {
	if d == nil || d.input == nil || !d.input.valid {
		return ErrInputInvalid
	}
	return nil
}

func (in *WindowInput) backend() *Backend {
	return &in.input.backend
}

func (d *DesktopInput) backend() *Backend {
	return &d.input.backend
}

// Move moves the pointer to client coordinates.
func (in *WindowInput) Move(x, y int32) error {
	if err := in.ready(); err != nil {
		return err
	}
	return moveWindow(in.window, x, y, false, in.backend())
}

// MoveRel moves the pointer by a client delta.
func (in *WindowInput) MoveRel(dx, dy int32) error {
	if err := in.ready(); err != nil {
		return err
	}
	return moveWindow(in.window, dx, dy, true, in.backend())
}

// Click clicks the left button at client coordinates.
func (in *WindowInput) Click(x, y int32) error {
	if err := in.ready(); err != nil {
		return err
	}
	return clickWindow(in.window, x, y, in.backend())
}

// ClickRight clicks the right button at client coordinates.
func (in *WindowInput) ClickRight(x, y int32) error {
	if err := in.ready(); err != nil {
		return err
	}
	return clickRightWindow(in.window, x, y, in.backend())
}

// ClickMiddle clicks the middle button at client coordinates.
func (in *WindowInput) ClickMiddle(x, y int32) error {
	if err := in.ready(); err != nil {
		return err
	}
	return clickMiddleWindow(in.window, x, y, in.backend())
}

// DoubleClick double-clicks the left button at client coordinates.
func (in *WindowInput) DoubleClick(x, y int32) error {
	if err := in.ready(); err != nil {
		return err
	}
	return doubleClickWindow(in.window, x, y, in.backend())
}

// Scroll posts a directed wheel event. BackendHID returns
// ErrDirectedScrollUnsupported and does not send a desktop wheel.
func (in *WindowInput) Scroll(x, y, delta int32) error {
	if err := in.ready(); err != nil {
		return err
	}
	return scrollWindow(in.window, x, y, delta, in.backend())
}

// KeyDown presses a key for this window.
func (in *WindowInput) KeyDown(key Key) error {
	if err := in.ready(); err != nil {
		return err
	}
	return keyDownWindow(in.window, key, in.backend())
}

// KeyUp releases a key for this window.
func (in *WindowInput) KeyUp(key Key) error {
	if err := in.ready(); err != nil {
		return err
	}
	return keyUpWindow(in.window, key, in.backend())
}

// Press presses and releases a key for this window.
func (in *WindowInput) Press(key Key, interval ...time.Duration) error {
	if err := in.ready(); err != nil {
		return err
	}
	hold, _, err := resolveInterval(interval, defaultPressHold)
	if err != nil {
		return err
	}
	return pressWindow(in.window, key, hold, in.backend())
}

// PressHotkey presses a key combination for this window.
func (in *WindowInput) PressHotkey(keys ...Key) error {
	if err := in.ready(); err != nil {
		return err
	}
	return pressHotkeyWindow(in.window, in.backend(), keys...)
}

// Type types text into this window.
func (in *WindowInput) Type(text string, interval ...time.Duration) error {
	if err := in.ready(); err != nil {
		return err
	}
	gap, explicit, err := resolveInterval(interval, keyboard.DefaultTypeGap)
	if err != nil {
		return err
	}
	return typeWindow(in.window, text, gap, explicit, in.backend())
}

// MoveMouseTo moves the pointer to virtual-desktop coordinates.
func (d *DesktopInput) MoveMouseTo(x, y int32) error {
	if err := d.ready(); err != nil {
		return err
	}
	return moveMouseTo(x, y, d.backend())
}

// ClickMouseAt moves to screen coordinates and clicks the left button.
func (d *DesktopInput) ClickMouseAt(x, y int32) error {
	if err := d.ready(); err != nil {
		return err
	}
	return clickMouseAt(x, y, d.backend())
}

// DoubleClickMouseAt moves to screen coordinates and double-clicks.
func (d *DesktopInput) DoubleClickMouseAt(x, y int32) error {
	if err := d.ready(); err != nil {
		return err
	}
	return doubleClickMouseAt(x, y, d.backend())
}

// ClickRightMouseAt moves to screen coordinates and clicks the right button.
func (d *DesktopInput) ClickRightMouseAt(x, y int32) error {
	if err := d.ready(); err != nil {
		return err
	}
	return clickRightMouseAt(x, y, d.backend())
}

// ClickMiddleMouseAt moves to screen coordinates and clicks the middle button.
func (d *DesktopInput) ClickMiddleMouseAt(x, y int32) error {
	if err := d.ready(); err != nil {
		return err
	}
	return clickMiddleMouseAt(x, y, d.backend())
}

// MouseDown presses a button at the current pointer position.
func (d *DesktopInput) MouseDown(button MouseButton) error {
	if err := d.ready(); err != nil {
		return err
	}
	return mouseDown(button, d.backend())
}

// MouseUp releases a button at the current pointer position.
func (d *DesktopInput) MouseUp(button MouseButton) error {
	if err := d.ready(); err != nil {
		return err
	}
	return mouseUp(button, d.backend())
}

// ClickMouse clicks the left button at the current pointer position.
func (d *DesktopInput) ClickMouse() error {
	if err := d.ready(); err != nil {
		return err
	}
	return clickMouse(d.backend())
}

// MoveMouseBy moves the pointer by a screen delta.
func (d *DesktopInput) MoveMouseBy(dx, dy int32) error {
	if err := d.ready(); err != nil {
		return err
	}
	return moveMouseBy(dx, dy, d.backend())
}

// MoveMouseToExact moves the pointer to an exact virtual-desktop point.
func (d *DesktopInput) MoveMouseToExact(x, y int32) error {
	if err := d.ready(); err != nil {
		return err
	}
	return moveMouseToExact(x, y, d.backend())
}

// ScrollWheel scrolls by whole notches at the pointer.
func (d *DesktopInput) ScrollWheel(steps int) error {
	if err := d.ready(); err != nil {
		return err
	}
	return scrollWheel(steps, d.backend())
}

// PressFor holds a global key.
func (d *DesktopInput) PressFor(key Key, duration time.Duration) error {
	if err := d.ready(); err != nil {
		return err
	}
	return pressFor(key, duration, d.backend())
}

// KeyDown presses a global key.
func (d *DesktopInput) KeyDown(key Key) error {
	if err := d.ready(); err != nil {
		return err
	}
	return keyDownGlobal(key, d.backend())
}

// KeyUp releases a global key.
func (d *DesktopInput) KeyUp(key Key) error {
	if err := d.ready(); err != nil {
		return err
	}
	return keyUpGlobal(key, d.backend())
}

// Press presses and releases a global key.
func (d *DesktopInput) Press(key Key, interval ...time.Duration) error {
	if err := d.ready(); err != nil {
		return err
	}
	hold, _, err := resolveInterval(interval, defaultPressHold)
	if err != nil {
		return err
	}
	return pressGlobal(key, hold, d.backend())
}

// PressHotkey presses a global key combination.
func (d *DesktopInput) PressHotkey(keys ...Key) error {
	if err := d.ready(); err != nil {
		return err
	}
	inputMutex.Lock()
	defer inputMutex.Unlock()
	cb, err := acquireBackend(d.backend())
	if err != nil {
		return err
	}
	return pressHotkeyWith(cb, keys...)
}

// Type types text with windowless keyboard input.
func (d *DesktopInput) Type(text string, interval ...time.Duration) error {
	if err := d.ready(); err != nil {
		return err
	}
	gap, explicit, err := resolveInterval(interval, keyboard.DefaultTypeGap)
	if err != nil {
		return err
	}
	return typeGlobal(text, gap, explicit, d.backend())
}

// PasteByClipboard pastes text into the current foreground window with
// Ctrl+V, using this executor's fixed backend. It does not read the global
// backend. The clipboard is still process-wide, the target is whatever window
// is foreground when the call runs, and no HWND is accepted. See the package
// PasteByClipboard for the clipboard format, restore, and consumption limits;
// this method has the same limits.
//
// A nil or zero receiver returns ErrInputInvalid before text validation, backend
// resolution, or the clipboard is touched. Empty text on a valid receiver
// returns ErrEmptyPasteText without taking the input lock or opening the
// clipboard.
//
// The fixed-backend path adds no wrapper-level initialization or availability
// check: HID initialization remains at NewInput(BackendHID), and lower-level
// runtime behavior is unchanged. A runtime backend error can therefore occur
// after the clipboard swap; restore is still attempted and errors are joined,
// so errors.Is continues to match each stage.
func (d *DesktopInput) PasteByClipboard(text string) error {
	if err := d.ready(); err != nil {
		return err
	}
	return pasteByClipboard(text, d.backend())
}
