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
*   [func KeyDown](#func-keydown)
*   [func KeyUp](#func-keyup)
*   [func Press](#func-press)
*   [func PressHotkey](#func-presshotkey)
*   [func Type](#func-type)
*   [func CaptureVirtualDesktop](#func-capturevirtualdesktop)
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
    *   [func (*Window) MoveRel](#func-window-moverel)
    *   [func (*Window) Press](#func-window-press)
    *   [func (*Window) PressHotkey](#func-window-presshotkey)
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

    // ErrPostMessageFailed implies the PostMessageW call returned 0 (e.g., queue full or invalid handle).
    ErrPostMessageFailed = errors.New("PostMessageW failed")
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
func Press(k Key) error
```
Press simulates a global key press (down then up) with a short delay.

### func PressHotkey

```go
func PressHotkey(keys ...Key) error
```
PressHotkey simulates a key combination (e.g., Ctrl+C). It presses keys in order, waits 50ms, then releases them in reverse order.

### func Type

```go
func Type(text string) error
```
Type simulates global text input by simulating keystrokes for each character.

### func CaptureVirtualDesktop

```go
func CaptureVirtualDesktop() (*image.RGBA, error)
```
CaptureVirtualDesktop (in package `screen`) captures the entire virtual desktop (all monitors) using GDI. It requires the process to be Per-Monitor DPI Aware. Returns an `*image.RGBA`.

## Types

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
Scroll performs a vertical mouse wheel scroll at the specified coordinates.

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

#### func (*Window) PressHotkey

```go
func (w *Window) PressHotkey(keys ...Key) error
```
PressHotkey presses a combination of keys in order and releases them in reverse order.

#### func (*Window) Type

```go
func (w *Window) Type(text string) error
```
Types a string, automatically handling Shift modifiers.

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
