# winput API Reference

Package `winput` provides a high-level interface for Windows background input automation.

## Index

*   [Variables](#variables)
*   [Constants](#constants)
*   [func EnablePerMonitorDPI](#func-enablepermonitordpi)
*   [func SetBackend](#func-setbackend)
*   [type Backend](#type-backend)
*   [type Key](#type-key)
    *   [func KeyFromRune](#func-keyfromrune)
*   [type Window](#type-window)
    *   [func FindByClass](#func-findbyclass)
    *   [func FindByPID](#func-findbypid)
    *   [func FindByTitle](#func-findbytitle)
    *   [func (*Window) Click](#func-window-click)
    *   [func (*Window) ClickRight](#func-window-clickright)
    *   [func (*Window) ClientRect](#func-window-clientrect)
    *   [func (*Window) ClientToScreen](#func-window-clienttoscreen)
    *   [func (*Window) DPI](#func-window-dpi)
    *   [func (*Window) DoubleClick](#func-window-doubleclick)
    *   [func (*Window) KeyDown](#func-window-keydown)
    *   [func (*Window) KeyUp](#func-window-keyup)
    *   [func (*Window) Move](#func-window-move)
    *   [func (*Window) Press](#func-window-press)
    *   [func (*Window) ScreenToClient](#func-window-screentoclient)
    *   [func (*Window) Type](#func-window-type)

---

## Variables

```go
var (
    ErrWindowNotFound     = errors.New("window not found")
    ErrUnsupportedKey     = errors.New("unsupported key")
    ErrBackendUnavailable = errors.New("backend unavailable")
    ErrPermissionDenied   = errors.New("permission denied")
)
```

## Constants

### Backend Constants

```go
const (
    // BackendMessage uses standard Windows Messages (PostMessage) for input.
    // It works in the background and does not require window focus.
    BackendMessage Backend = iota

    // BackendHID uses the Interception driver to simulate hardware input.
    // It requires the Interception driver to be installed on the system.
    // Input via this backend will move the physical cursor and is indistinguishable form hardware input.
    BackendHID
)
```

### Key Constants
Common keyboard scan codes.

```go
const (
    KeyEsc, KeyEnter, KeySpace, KeyTab, KeyBkSp Key = ...
    KeyShift, KeyCtrl, KeyAlt, KeyCaps          Key = ...
    KeyF1 .. KeyF12                             Key = ...
    KeyA .. KeyZ                                Key = ...
    Key0 .. Key9                                Key = ...
    // ... and more standard keys
)
```

## Functions

### func EnablePerMonitorDPI

```go
func EnablePerMonitorDPI() error
```
EnablePerMonitorDPI sets the current process to be Per-Monitor (v2) DPI aware. This ensures that coordinate calculations (ScreenToClient/ClientToScreen) are accurate on high-DPI setups. It is recommended to call this at the start of your program.

### func SetBackend

```go
func SetBackend(b Backend)
```
SetBackend configures the global input injection method. The default is `BackendMessage`.
If `BackendHID` is selected, initialization checks (driver presence) are deferred until the first input action is attempted.

## Types

### type Backend

```go
type Backend int
```
Backend represents the underlying mechanism used for input injection.

### type Key

```go
type Key = uint16
```
Key represents a hardware scan code. It avoids using Virtual Keys (VK) to ensure better compatibility with low-level hooks and games.

#### func KeyFromRune

```go
func KeyFromRune(r rune) (Key, bool)
```
KeyFromRune attempts to map a unicode character (rune) to a corresponding scan code `Key`. It supports basic ASCII characters. Returns false if the rune cannot be mapped.

### type Window

```go
type Window struct {
    HWND uintptr
}
```
Window represents a target window for automation. It encapsulates the window handle (HWND) and provides methods for input and coordinate management.

#### func FindByTitle

```go
func FindByTitle(title string) (*Window, error)
```
FindByTitle searches for a top-level window matching the exact title. Returns `ErrWindowNotFound` if no match is found.

#### func FindByClass

```go
func FindByClass(class string) (*Window, error)
```
FindByClass searches for a top-level window matching the class name (e.g., "Notepad", "Chrome_WidgetWin_1").

#### func FindByPID

```go
func FindByPID(pid uint32) ([]*Window, error)
```
FindByPID returns all top-level windows belonging to the specified Process ID.

#### func (*Window) Move

```go
func (w *Window) Move(x, y int32) error
```
Move moves the mouse cursor to the specified coordinates **relative to the window's client area**.
- In `BackendMessage`: It posts a `WM_MOUSEMOVE` message.
- In `BackendHID`: It calculates the absolute screen position and physically moves the mouse cursor (with human-like smoothing).

#### func (*Window) Click

```go
func (w *Window) Click(x, y int32) error
```
Click performs a left mouse button click at the specified client coordinates. It automatically moves the cursor to the target location first.

#### func (*Window) ClickRight

```go
func (w *Window) ClickRight(x, y int32) error
```
ClickRight performs a right mouse button click at the specified client coordinates.

#### func (*Window) DoubleClick

```go
func (w *Window) DoubleClick(x, y int32) error
```
DoubleClick performs a left mouse button double-click.

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
func (w *Window) Press(key Key) error
```
Press simulates a full keystroke (KeyDown followed by KeyUp).
In `BackendHID`, a random delay is inserted between down and up events to simulate human speed.

#### func (*Window) Type

```go
func (w *Window) Type(text string) error
```
Type types a string of text into the window. It maps characters to keys and presses them sequentially.

#### func (*Window) DPI

```go
func (w *Window) DPI() (uint32, error)
```
DPI returns the Dots Per Inch (DPI) setting for the window. Standard DPI is 96.

#### func (*Window) ClientRect

```go
func (w *Window) ClientRect() (width, height int32, err error)
```
ClientRect returns the width and height of the window's client area (excluding borders and title bar).

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
