# winput API Reference

Package `winput` provides a high-level interface for Windows background input automation.

## Index

*   [Variables](#variables)
*   [Constants](#constants)
*   [func EnablePerMonitorDPI](#func-enablepermonitordpi)
*   [func GetCursorPos](#func-getcursorpos)
*   [func SetBackend](#func-setbackend)
*   [func SetHIDLibraryPath](#func-sethidlibrarypath)
*   [func MoveMouseTo](#func-movemouseto)
*   [func ClickMouseAt](#func-clickmouseat)
*   [func MouseDown](#func-mousedown)
*   [func MouseUp](#func-mouseup)
*   [func ClickMouse](#func-clickmouse)
*   [func MoveMouseBy](#func-movemouseby)
*   [func MoveMouseToExact](#func-movemousetoexact)
*   [func ScrollWheel](#func-scrollwheel)
*   [func PressFor](#func-pressfor)
*   [func KeyDown](#func-keydown)
*   [func KeyUp](#func-keyup)
*   [func Press](#func-press)
*   [func PressHotkey](#func-presshotkey)
*   [func PasteByClipboard](#func-pastebyclipboard)
*   [func Type](#func-type)
*   [func CaptureVirtualDesktop](#func-capturevirtualdesktop)
*   [func CaptureRegion](#func-captureregion)
*   [type Backend](#type-backend)
*   [type MouseButton](#type-mousebutton)
*   [type Key](#type-key)
    *   [func KeyFromRune](#func-keyfromrune)
*   [type Window](#type-window)
    *   [func FindByClass](#func-findbyclass)
    *   [func FindByPID](#func-findbypid)
    *   [func FindByProcessName](#func-findbyprocessname)
    *   [func FindByTitle](#func-findbytitle)
    *   [func (*Window) Click](#func-window-click)
    *   [func (*Window) ClickMiddle](#func-window-clickmiddle)
    *   [func (*Window) ClickRight](#func-window-clickright)
    *   [func (*Window) ClientRect](#func-window-clientrect)
    *   [func (*Window) ClientToScreen](#func-window-clienttoscreen)
    *   [func (*Window) ClientInfo](#func-window-clientinfo)
    *   [func (*Window) DPI](#func-window-dpi)
    *   [func (*Window) DoubleClick](#func-window-doubleclick)
    *   [func (*Window) KeyDown](#func-window-keydown)
    *   [func (*Window) KeyUp](#func-window-keyup)
    *   [func (*Window) Move](#func-window-move)
    *   [func (*Window) MoveRel](#func-window-moverel)
    *   [func (*Window) Press](#func-window-press)
    *   [func (*Window) PressHotkey](#func-window-presshotkey)
    *   [func (*Window) ScreenToClient](#func-window-screentoclient)
    *   [func (*Window) Scroll](#func-window-scroll)
    *   [func (*Window) Text](#func-window-text)
    *   [func (*Window) Type](#func-window-type)
    *   [func (*Window) Value](#func-window-value)

---

## Variables

```go
var (
    // ErrWindowNotFound implies the target window could not be located by Title, Class, or PID.
    ErrWindowNotFound = errors.New("window not found")

    // ErrWindowGone implies the window handle is no longer valid.
    ErrWindowGone = errors.New("window is gone or invalid")

    // ErrWindowNotVisible implies the window is hidden or minimized.
    ErrWindowNotVisible = errors.New("window is not visible")

    // ErrUnsupportedKey aliases keyboard.ErrUnsupportedKey.
    // The text is "unsupported key or character". It is not a separate sentinel.
    ErrUnsupportedKey = keyboard.ErrUnsupportedKey

    // ErrBackendUnavailable implies the selected backend (e.g. HID) failed to initialize.
    ErrBackendUnavailable = errors.New("input backend unavailable")

    // ErrDriverNotInstalled specific to BackendHID, implies the Interception driver is missing or not accessible.
    ErrDriverNotInstalled = errors.New("interception driver not installed or accessible")

    // ErrDLLLoadFailed implies interception.dll could not be loaded.
    ErrDLLLoadFailed = errors.New("failed to load interception library")

    // ErrPermissionDenied implies the operation failed due to system privilege restrictions (e.g. UIPI).
    ErrPermissionDenied = errors.New("permission denied")

    // ErrPostMessageFailed aliases window.ErrPostMessageFailed.
    ErrPostMessageFailed = window.ErrPostMessageFailed

    // ErrReadTextFailed aliases window.ErrReadTextFailed.
    ErrReadTextFailed = window.ErrReadTextFailed

    // ErrMoveNotConverged aliases hid.ErrMoveNotConverged.
    ErrMoveNotConverged = hid.ErrMoveNotConverged

    // ErrOutsideVirtualDesktop aliases hid.ErrOutsideVirtualDesktop.
    ErrOutsideVirtualDesktop = hid.ErrOutsideVirtualDesktop

    // ErrSendInputFailed means SendInput inserted zero events.
    ErrSendInputFailed = errors.New("SendInput inserted no events")

    // ErrSendInputPartial means SendInput inserted only the key-down event,
    // and the compensating key-up was inserted successfully.
    ErrSendInputPartial = errors.New("SendInput partially inserted and was repaired")

    // ErrSendInputStuck means SendInput inserted only the key-down event,
    // and the compensating key-up could not be inserted.
    ErrSendInputStuck = errors.New("SendInput partially inserted and could not be repaired")

    // ErrDirectedScrollUnsupported means the HID backend cannot direct a wheel
    // event to a specific window.
    ErrDirectedScrollUnsupported = errors.New("HID backend does not support directed scrolling")

    // ErrInvalidMouseButton means the MouseButton value is not one of the exported identities.
    ErrInvalidMouseButton = errors.New("invalid mouse button")

    // ErrInvalidScrollDelta aliases mouse.ErrInvalidScrollDelta
    // ("scroll delta out of int16 range"). Window.Scroll on BackendMessage
    // returns it. hid.ErrInvalidScrollDelta is a different sentinel.
    ErrInvalidScrollDelta = mouse.ErrInvalidScrollDelta

    // ErrInvalidScrollSteps means ScrollWheel steps are zero or outside [-273, 273].
    ErrInvalidScrollSteps = errors.New("scroll steps must be non-zero and within [-273, 273]")

    // ErrInvalidPressDuration means PressFor was given less than 10ms.
    ErrInvalidPressDuration = errors.New("press duration must be at least 10ms")

    // ErrInvalidInterval aliases keyboard.ErrInvalidInterval
    // ("interval must be a single non-negative duration"). Press and Type
    // return it when the optional interval is negative or more than one
    // argument is supplied. It is deliberately distinct from
    // ErrInvalidPressDuration: an interval of 0 is legal.
    ErrInvalidInterval = keyboard.ErrInvalidInterval

    // ErrEmptyPasteText means PasteByClipboard was called with an empty string.
    // The clipboard is not opened and no key is sent.
    ErrEmptyPasteText = errors.New("paste text is empty")

    // ErrClipboardFailed aliases window.ErrClipboardFailed.
    // Means a clipboard open, read, write, or restore failed. The error text
    // never includes clipboard contents.
    ErrClipboardFailed = window.ErrClipboardFailed
)
```

## Screen Package (`github.com/rpdg/winput/screen`)

### func ImageToVirtual

```go
func ImageToVirtual(imageX, imageY int32) (int32, int32)
```
ImageToVirtual converts coordinates from a "Full Virtual Desktop Screenshot" (OpenCV match) to actual Windows Virtual Desktop coordinates usable by `MoveMouseTo`.
Requires the image to be a capture of the **entire** virtual desktop (origin 0,0 of the image corresponds to the top-left of the virtual canvas).

### func VirtualBounds

```go
func VirtualBounds() Rect
```
VirtualBounds returns the bounding rectangle of the entire virtual desktop.

### func Monitors

```go
func Monitors() ([]Monitor, error)
```
Monitors returns a list of all active monitors and their geometries.

## Constants

### Backend Constants

```go
const (
    // BackendMessage uses standard Windows Messages (PostMessage) for input.
    BackendMessage Backend = iota

    // BackendHID uses the Interception driver to simulate hardware input.
    BackendHID
)
```

### Key Constants

`Key` is a composite value. The low 8 bits are the Set 1 scan code. Keys that need the E0 prefix set the high byte to `0xE0`. Do not treat the whole value as a scan code: `0x48` is numpad 8 (`KeyNumpad8`), and arrow up is `0xE048` (`KeyArrowUp`).

```go
const (
    KeyEsc, KeyEnter, KeySpace, KeyTab, KeyBkSp Key = ...
    KeyShift, KeyCtrl, KeyAlt, KeyCaps          Key = ...
    KeyLeftCtrl                                 Key = 0x1D
    KeyRightCtrl                                Key = 0xE01D
    KeyLeftAlt                                  Key = 0x38
    KeyRightAlt                                 Key = 0xE038
    KeyNumpadDivide, KeyDivide                  Key = 0xE035 // KeyDivide aliases KeyNumpadDivide
    KeyLWin                                     Key = 0xE05B
    KeyRWin                                     Key = 0xE05C
    KeyF1 .. KeyF12                             Key = ...
    KeyA .. KeyZ                                Key = ...
    Key0 .. Key9                                Key = ...
    KeyArrowUp, KeyArrowDown, KeyLeft, KeyRight Key = ... // high byte 0xE0
    KeyHome, KeyEnd, KeyPageUp, KeyPageDown     Key = ...
    KeyInsert, KeyDelete                        Key = ...
)
```

## Functions

### func EnablePerMonitorDPI

```go
func EnablePerMonitorDPI() error
```
EnablePerMonitorDPI sets the current process to be Per-Monitor (v2) DPI aware.

### func GetCursorPos

```go
func GetCursorPos() (int32, int32, error)
```
GetCursorPos returns the current absolute screen coordinates of the mouse cursor.

### func SetBackend

```go
func SetBackend(b Backend) error
```
SetBackend sets the input simulation backend. If `BackendHID` is selected, it attempts to initialize the driver immediately and returns an error if it fails (e.g. driver not installed).

### func SetHIDLibraryPath

```go
func SetHIDLibraryPath(path string)
```
SetHIDLibraryPath sets the custom path for `interception.dll`.

### func MoveMouseTo

```go
func MoveMouseTo(x, y int32) error
```
MoveMouseTo moves the mouse cursor to the absolute screen coordinates (Virtual Desktop).

### func ClickMouseAt

```go
func ClickMouseAt(x, y int32) error
```
ClickMouseAt moves the mouse to the specified screen coordinates and performs a left click.

### func ClickRightMouseAt

```go
func ClickRightMouseAt(x, y int32) error
```
ClickRightMouseAt moves the mouse to the specified screen coordinates and performs a right click.

### func ClickMiddleMouseAt

```go
func ClickMiddleMouseAt(x, y int32) error
```
ClickMiddleMouseAt moves the mouse to the specified screen coordinates and performs a middle click.

### func MouseDown

```go
func MouseDown(button MouseButton) error
```

MouseDown presses the specified button at the current cursor position without moving. Invalid button identities return `ErrInvalidMouseButton`. The input mutex covers this call only; a down/move/up sequence across separate calls is not atomic.

### func MouseUp

```go
func MouseUp(button MouseButton) error
```

MouseUp releases the specified button at the current cursor position without moving. Invalid button identities return `ErrInvalidMouseButton`. The input mutex covers this call only; a down/move/up sequence across separate calls is not atomic.

### func ClickMouse

```go
func ClickMouse() error
```

ClickMouse performs a left click at the current cursor position without moving. Button flags use the same mapping as `MouseDown` and `MouseUp`. Under `BackendMessage` it uses `mouse_event`, which has no return value, so button-event delivery failures are not observable. Under `BackendHID` a failed release is retried once, and both failures keep the first error.

### func MoveMouseBy

```go
func MoveMouseBy(dx, dy int32) error
```

MoveMouseBy moves the cursor by a delta. Under `BackendHID` this is one relative stroke. Under `BackendMessage` it reads the cursor, adds the delta using `int64`, and calls `SetCursorPos`; success means the system accepted the resulting coordinate.

### func MoveMouseToExact

```go
func MoveMouseToExact(x, y int32) error
```

MoveMouseToExact performs one absolute move. It rejects points outside the virtual desktop with `ErrOutsideVirtualDesktop`. The HID path sends one absolute stroke over the virtual desktop, waits 12ms, and verifies the process-space cursor within one pixel; failure returns `ErrMoveNotConverged`. The one-pixel check is a reading in the process coordinate space, not a physical-pixel proof (RISK-001). The message path calls `SetCursorPos`, where success means the system accepted the call; it does not use that normalization and does not wait.

### func ScrollWheel

```go
func ScrollWheel(steps int) error
```

ScrollWheel scrolls by whole wheel steps. Positive steps scroll forward, negative steps backward. Zero and values outside `[-273, 273]` return `ErrInvalidScrollSteps` before multiplication. The message backend uses `mouse_event`; the HID backend sends one wheel stroke. Neither moves the cursor.

### func PressFor

```go
func PressFor(key Key, duration time.Duration) error
```

PressFor holds a global key for the requested duration. Durations below 10ms return `ErrInvalidPressDuration`. The message backend uses scan-code `SendInput`; the HID backend sends a hardware key stroke. A failed key-up is retried once.

### func DoubleClickMouseAt

```go
func DoubleClickMouseAt(x, y int32) error
```
DoubleClickMouseAt moves the mouse to the specified screen coordinates and performs a left double-click.

### func KeyDown

```go
func KeyDown(k Key) error
```
KeyDown simulates a global key down event.

### func KeyUp

```go
func KeyUp(k Key) error
```
KeyUp simulates a global key up event.

### func Press

```go
func Press(k Key, interval ...time.Duration) error
```
Press simulates a global key press (down then up). The optional `interval` is
the hold time (delay between a successful key-down and the release) and is
honored on both backends; without it the hold is 30ms. `Press(key, 0)` does
not sleep between down and up. A negative interval, or more than one interval,
returns `ErrInvalidInterval` before any key is pressed.

### func PressHotkey

```go
func PressHotkey(keys ...Key) error
```
PressHotkey simulates a key combination (e.g., Ctrl+C). It presses keys in order, waits 50ms, then releases them in reverse order.

### func Type

```go
func Type(text string, interval ...time.Duration) error
```
Type simulates global text input by simulating keystrokes for each character.
The optional `interval` is the character gap on both backends. On the message
backend the gap is slept between runes (30ms by default); the last rune is not
followed by an extra sleep. On HID the fixed per-character hold is unchanged
and an explicit interval is slept between character releases; without an
argument no extra gap is added after each release. A negative interval, or more
than one interval, returns `ErrInvalidInterval` before any character is sent.

### func PasteByClipboard

```go
func PasteByClipboard(text string) error
```
PasteByClipboard pastes text into the current foreground window using Ctrl+V.
It takes no HWND: the keystrokes go to whatever window is in the foreground
when the call runs.

A nil return does **not** mean the foreground program has received the new
text. It only means the Ctrl+V events were queued and the previous Unicode
clipboard text was restored without error. `SendInput` returns once the events
are queued and does not wait for the foreground to process the paste, so there
is no synchronization point proving the foreground read the clipboard before
it was restored. A non-nil return does not prove the keys were not inserted
either: the key can have succeeded while the restore failed, in which case
`errors.Is` can recognize both `ErrClipboardFailed` and `ErrSendInputFailed`
on the same error. To know whether a control accepted the text, observe that
control separately.

The clipboard is temporarily replaced and only `CF_UNICODETEXT` is put back.
If the system had no Unicode format (including a clipboard that only held an
image, or a `CF_TEXT` the system could convert), a bare empty Unicode buffer is
restored and the original non-text formats do not return. New text has a lone
`\n` normalized to `\r\n` before it is written; restored text is written
byte-for-byte as it was read. Empty text fails with `ErrEmptyPasteText` before
the clipboard is opened and before any key is sent. PasteByClipboard is not a
replacement for `Type`; controls that reject paste (for example password
fields) will not succeed.


### func CaptureVirtualDesktop

```go
func CaptureVirtualDesktop() (*image.RGBA, error)
```
CaptureVirtualDesktop (in package `screen`) captures the entire virtual desktop (all monitors) using GDI. It requires the process to be Per-Monitor DPI Aware. Returns an `*image.RGBA`.

### func CaptureRegion

```go
func CaptureRegion(x, y, w, h int32) (*image.RGBA, error)
```
CaptureRegion (in package `screen`) captures a specific region of the virtual desktop.
`x`, `y` are Virtual Desktop coordinates (allowed to be negative).
`w`, `h` are pixel dimensions.
It internally calls `CaptureVirtualDesktop`, converts coordinates, and performs a safe crop.

## Types

### type MouseButton

```go
type MouseButton uint8
```

MouseButton identifies a mouse button. The exported identities are `MouseLeft`, `MouseRight`, and `MouseMiddle`. Driver-specific values are mapped internally.

### type Window

#### func FindByTitle

```go
func FindByTitle(title string) (*Window, error)
```
FindByTitle searches for a top-level window matching the exact title.

#### func FindByClass

```go
func FindByClass(class string) (*Window, error)
```
FindByClass searches for a top-level window matching the class name.

#### func FindByPID

```go
func FindByPID(pid uint32) ([]*Window, error)
```
FindByPID returns all top-level windows belonging to the specified Process ID.

#### func FindByProcessName

```go
func FindByProcessName(name string) ([]*Window, error)
```
FindByProcessName returns all top-level windows belonging to the process with the given executable name.

#### func (*Window) FindChildByClass

```go
func (w *Window) FindChildByClass(class string) (*Window, error)
```
FindChildByClass searches for a child window with the specified class name (e.g. "Edit" inside Notepad).

#### func (*Window) Move

```go
func (w *Window) Move(x, y int32) error
```
Move moves the mouse cursor to the specified coordinates relative to the window's client area.

#### func (*Window) MoveRel

```go
func (w *Window) MoveRel(dx, dy int32) error
```
MoveRel moves the mouse cursor relative to its current position.

#### func (*Window) Click

```go
func (w *Window) Click(x, y int32) error
```
Click performs a left mouse button click at the specified client coordinates.

#### func (*Window) ClickRight

```go
func (w *Window) ClickRight(x, y int32) error
```
ClickRight performs a right mouse button click at the specified client coordinates.

#### func (*Window) ClickMiddle

```go
func (w *Window) ClickMiddle(x, y int32) error
```
ClickMiddle performs a middle mouse button click at the specified client coordinates.

#### func (*Window) DoubleClick

```go
func (w *Window) DoubleClick(x, y int32) error
```
DoubleClick performs a left mouse button double-click.

#### func (*Window) Scroll

```go
func (w *Window) Scroll(x, y int32, delta int32) error
```
Scroll performs a vertical mouse wheel scroll at the specified client coordinates.

Under `BackendMessage` it posts `WM_MOUSEWHEEL` to this window. `delta == 0` is a no-op. Any other delta must fit in `int16`, or the call returns `ErrInvalidScrollDelta` (an alias of `mouse.ErrInvalidScrollDelta`). Under `BackendHID`, `delta` is not validated and the method always returns `ErrDirectedScrollUnsupported`, because a hardware wheel event cannot be addressed to a specific window. Use `ScrollWheel` for hardware-level scrolling.

#### func (*Window) Text

```go
func (w *Window) Text() (string, error)
```
Text returns the current text of the target window/control using standard Win32 text retrieval. It is primarily intended for controls such as `Edit` and `RichEdit`.

#### func (*Window) KeyDown

```go
func (w *Window) KeyDown(key Key) error
```
KeyDown sends a key down event to the window.

#### func (*Window) KeyUp

```go
func (w *Window) KeyUp(key Key) error
```
KeyUp sends a key up event to the window.

#### func (*Window) Press

```go
func (w *Window) Press(key Key, interval ...time.Duration) error
```
Press simulates a full keystroke (KeyDown followed by KeyUp). The optional
`interval` is the hold time (delay between a successful key-down and the
release) on both backends; without it the hold is 30ms. `Press(key, 0)` does
not sleep between down and up. A negative interval, or more than one interval,
returns `ErrInvalidInterval` before any key is pressed.

#### func (*Window) PressHotkey

```go
func (w *Window) PressHotkey(keys ...Key) error
```
PressHotkey presses a combination of keys in order and releases them in reverse order.
When used on a window with `BackendMessage`, combinations that include modifier keys
such as `Ctrl`, `Alt`, or `Shift` are sent via foreground keyboard events instead of
pure `PostMessage`, because many applications do not recognize modifier state reliably
through window messages alone.

#### func (*Window) Type

```go
func (w *Window) Type(text string, interval ...time.Duration) error
```
Types a string, automatically handling Shift modifiers. The optional `interval`
is the character gap: on the message backend it is slept between runes (30ms
by default, via `keyboard.TypeWithInterval`) and not after the last rune; on
HID the fixed per-character hold is unchanged and an explicit interval is
slept between character releases. A negative interval, or more than one
interval, returns `ErrInvalidInterval` before any character is sent.

#### func (*Window) Value

```go
func (w *Window) Value() (string, error)
```
Value returns the current best-effort textual value of the target window/control. It first tries the Win32 text path used by `Text()`, then falls back to Windows UI Automation for modern controls when needed.

#### func (*Window) DPI

```go
func (w *Window) DPI() (uint32, uint32, error)
```
DPI returns the horizontal and vertical DPI for the window.

#### func (*Window) ClientRect

```go
func (w *Window) ClientRect() (width, height int32, err error)
```
ClientRect returns the width and height of the window's client area.

#### func (*Window) ScreenToClient

```go
func (w *Window) ScreenToClient(x, y int32) (cx, cy int32, err error)
```
ScreenToClient converts screen-relative coordinates to window-client-relative coordinates.

#### func (*Window) ClientToScreen

```go
func (w *Window) ClientToScreen(x, y int32) (sx, sy int32, err error)
```
ClientToScreen converts window-client-relative coordinates to screen-relative coordinates.

#### func (*Window) ClientInfo

```go
func (w *Window) ClientInfo() (x, y, width, height int32, err error)
```
ClientInfo returns the client-area rectangle as screen coordinates: the screen
position of the client-origin (0, 0) plus the client width and height. It
excludes the title bar and borders.

The order is fixed: `ClientToScreen(0, 0)` first, then `ClientRect`. If either
call fails, all four integers are zero and the error is returned; a failed
first call does not reach the second one. A minimized window follows the
existing `"window is minimized"` error.

The coordinates use the DPI awareness the calling thread already has. To align
`ClientInfo` with `screen.CaptureRegion`, the process must run with the same
DPI awareness as the screenshot; `ClientInfo` does not call
`EnablePerMonitorDPI`. The four values come from two independent Win32 calls
and are not an atomic snapshot.

Use `EnablePerMonitorDPI` only at process startup; DPI awareness cannot be
changed reliably later.

