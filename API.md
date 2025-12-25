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
    *   [func FindByProcessName](#func-findbyprocessname)
    *   [func FindByTitle](#func-findbytitle)
    *   [func (*Window) Click](#func-window-click)
    *   [func (*Window) ClickMiddle](#func-window-clickmiddle)
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
    *   [func (*Window) Scroll](#func-window-scroll)
    *   [func (*Window) Type](#func-window-type)

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

    // ErrUnsupportedKey implies the character cannot be mapped to a key.
    ErrUnsupportedKey = errors.New("unsupported key or character")

    // ErrBackendUnavailable implies the selected backend (e.g. HID) failed to initialize.
    ErrBackendUnavailable = errors.New("input backend unavailable")

    // ErrDriverNotInstalled specific to BackendHID, implies the Interception driver is missing or not accessible.
    ErrDriverNotInstalled = errors.New("interception driver not installed or accessible")

    // ErrDLLLoadFailed implies interception.dll could not be loaded.
    ErrDLLLoadFailed = errors.New("failed to load interception library")

    // ErrPermissionDenied implies the operation failed due to system privilege restrictions (e.g. UIPI).
    ErrPermissionDenied = errors.New("permission denied")
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
    KeyArrowUp, KeyArrowDown, KeyLeft, KeyRight Key = ...
    KeyHome, KeyEnd, KeyPageUp, KeyPageDown     Key = ...
    KeyInsert, KeyDelete                        Key = ...
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

### func SetHIDLibraryPath

```go
func SetHIDLibraryPath(path string)
```
SetHIDLibraryPath sets the custom path for `interception.dll`.
By default, winput searches for the DLL in the system PATH or current directory.
This must be called **before** enabling `BackendHID`.

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

#### func FindByProcessName

```go
func FindByProcessName(name string) ([]*Window, error)
```
FindByProcessName returns all top-level windows belonging to the process with the given executable name (e.g. "notepad.exe").

#### func FindByTitle

#### func (*Window) Move

```go
func (w *Window) Move(x, y int32) error
```
Move moves the mouse cursor to the specified coordinates **relative to the window's client area**.
- **BackendMessage**: Posts a `WM_MOUSEMOVE` message (Instant).
- **BackendHID**: Calculates screen position and physically moves the cursor using a human-like trajectory. **This operation is synchronous and blocking.**

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
Scroll performs a vertical mouse wheel scroll at the specified coordinates.
`delta` indicates the scroll amount; 120 is one standard wheel "click". Positive values scroll forward/up, negative values scroll backward/down.

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
Types a string, automatically handling Shift modifiers for uppercase and symbols.


#### func (*Window) DPI

```go
func (w *Window) DPI() (uint32, error)
```
DPI returns the Dots Per Inch (DPI) setting for the window. Standard DPI is 96.
It attempts to use Per-Monitor V2 API, falling back to System DPI or GDI DeviceCaps on older systems.

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