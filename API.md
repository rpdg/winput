# winput API Reference

Package `winput` provides a high-level interface for Windows background input automation.

## Index

*   [Variables](#variables)
*   [Constants](#constants)
    *   [Backend Constants](#backend-constants)
    *   [MouseButton Constants](#mousebutton-constants)
    *   [Key Constants](#key-constants)
    *   [Probe Constants](#probe-constants)
*   [Functions](#functions)
    *   [func EnablePerMonitorDPI](#func-enablepermonitordpi)
    *   [func GetCursorPos](#func-getcursorpos)
    *   [func SetBackend](#func-setbackend)
    *   [func SetHIDLibraryPath](#func-sethidlibrarypath)
    *   [func MoveMouseTo](#func-movemouseto)
    *   [func ClickMouseAt](#func-clickmouseat)
    *   [func ClickRightMouseAt](#func-clickrightmouseat)
    *   [func ClickMiddleMouseAt](#func-clickmiddlemouseat)
    *   [func DoubleClickMouseAt](#func-doubleclickmouseat)
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
    *   [func Type](#func-type)
    *   [func KeyFromRune](#func-keyfromrune)
    *   [func PasteByClipboard](#func-pastebyclipboard)
    *   [func ProbeTarget](#func-probetarget)
    *   [func ProbeHID](#func-probehid)
    *   [func NewInput](#func-newinput)
    *   [func FindByTitle](#func-findbytitle)
    *   [func FindByClass](#func-findbyclass)
    *   [func FindByPID](#func-findbypid)
    *   [func FindByProcessName](#func-findbyprocessname)
*   [Types](#types)
    *   [type Backend](#type-backend)
    *   [type MouseButton](#type-mousebutton)
    *   [type Key](#type-key)
    *   [type Window](#type-window)
        *   [func (*Window) FindChildByClass](#func-window-findchildbyclass)
        *   [func (*Window) IsValid](#func-window-isvalid)
        *   [func (*Window) IsVisible](#func-window-isvisible)
        *   [func (*Window) Move](#func-window-move)
        *   [func (*Window) MoveRel](#func-window-moverel)
        *   [func (*Window) Click](#func-window-click)
        *   [func (*Window) ClickRight](#func-window-clickright)
        *   [func (*Window) ClickMiddle](#func-window-clickmiddle)
        *   [func (*Window) DoubleClick](#func-window-doubleclick)
        *   [func (*Window) Scroll](#func-window-scroll)
        *   [func (*Window) Text](#func-window-text)
        *   [func (*Window) Value](#func-window-value)
        *   [func (*Window) KeyDown](#func-window-keydown)
        *   [func (*Window) KeyUp](#func-window-keyup)
        *   [func (*Window) Press](#func-window-press)
        *   [func (*Window) PressHotkey](#func-window-presshotkey)
        *   [func (*Window) Type](#func-window-type)
        *   [func (*Window) DPI](#func-window-dpi)
        *   [func (*Window) ClientRect](#func-window-clientrect)
        *   [func (*Window) ClientInfo](#func-window-clientinfo)
        *   [func (*Window) ScreenToClient](#func-window-screentoclient)
        *   [func (*Window) ClientToScreen](#func-window-clienttoscreen)
    *   [type Input](#type-input)
        *   [func (*Input) ForWindow](#func-input-forwindow)
        *   [func (*Input) Desktop](#func-input-desktop)
    *   [type WindowInput](#type-windowinput)
        *   [WindowInput Methods](#windowinput-methods)
    *   [type DesktopInput](#type-desktopinput)
        *   [DesktopInput Methods](#desktopinput-methods)
        *   [func (*DesktopInput) PasteByClipboard](#func-desktopinput-pastebyclipboard)
    *   [type TargetProbe](#type-targetprobe)
    *   [type TargetFeatures](#type-targetfeatures)
    *   [type IntegrityRelation](#type-integrityrelation)
    *   [type UIAccess](#type-uiaccess)
    *   [type HIDStatus](#type-hidstatus)
    *   [type ProbeWarning](#type-probewarning)
*   [Screen Package (`github.com/rpdg/winput/screen`)](#screen-package-githubcomrpdgwinputscreen)
    *   [type Point](#type-point)
    *   [type Rect](#type-rect)
    *   [type Monitor](#type-monitor)
    *   [type CaptureOptions](#type-captureoptions)
    *   [func VirtualBounds](#func-virtualbounds)
    *   [func Monitors](#func-monitors)
    *   [func ImageToVirtual](#func-imagetovirtual)
    *   [func CaptureVirtualDesktop](#func-capturevirtualdesktop)
    *   [func CaptureVirtualDesktopWithOptions](#func-capturevirtualdesktopwithoptions)
    *   [func CaptureRegion](#func-captureregion)
    *   [func CaptureRegionWithOptions](#func-captureregionwithoptions)

---

## Variables

```go
var (
    // ErrWindowNotFound implies the target window could not be located by Title, Class, or PID.
    ErrWindowNotFound = errors.New("window not found")

    // ErrInvalidLookupString means a title or class contains an embedded NUL.
    // It is also wrapped with ErrWindowNotFound. errors.Is matches both.
    ErrInvalidLookupString = window.ErrInvalidLookupString

    // ErrWindowGone implies the window handle is no longer valid.
    ErrWindowGone = errors.New("window is gone or invalid")

    // ErrTargetChanged means the window handle was still valid, but its
    // process, thread, or a class name already read at the start did not match
    // at the end of a probe.
    ErrTargetChanged = errors.New("target window changed during probe")

    // ErrInputInvalid means an executor or facade was not produced by a
    // successful NewInput, ForWindow, or Desktop call.
    ErrInputInvalid = errors.New("input executor is invalid")

    // ErrInvalidBackend means NewInput was given a value other than
    // BackendMessage or BackendHID. BackendMessage itself is legal.
    ErrInvalidBackend = errors.New("invalid input backend")

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

    // ErrTargetNotForeground means a window key-down on BackendHID did not send
    // because the window's top-level ancestor was not the foreground window.
    ErrTargetNotForeground = errors.New("target window is not the foreground window")

    // ErrInvalidMouseButton means the MouseButton value is not one of the exported identities.
    ErrInvalidMouseButton = errors.New("invalid mouse button")

    // ErrInvalidMouseCoordinate means a mouse message coordinate does not fit
    // in the signed 16-bit halves of lParam. The message is not sent.
    ErrInvalidMouseCoordinate = mouse.ErrInvalidMouseCoordinate

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

---

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

### MouseButton Constants

```go
const (
    MouseLeft MouseButton = iota + 1
    MouseRight
    MouseMiddle
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

### Probe Constants

```go
// IntegrityRelation represents the caller's view of the target process integrity.
const (
    IntegrityUnknown      IntegrityRelation = "unknown"
    IntegrityTargetHigher IntegrityRelation = "target_higher"
    IntegrityEqual        IntegrityRelation = "equal"
    IntegrityTargetLower  IntegrityRelation = "target_lower"
)

// UIAccess represents the caller token's UIAccess flag.
const (
    UIAccessUnknown  UIAccess = "unknown"
    UIAccessEnabled  UIAccess = "enabled"
    UIAccessDisabled UIAccess = "disabled"
)

// HIDStatus represents the result of initializing this process's HID backend.
const (
    HIDUnknown            HIDStatus = "unknown"
    HIDInitialized        HIDStatus = "initialized"
    HIDDLLUnavailable     HIDStatus = "dll_unavailable"
    HIDDriverUnavailable  HIDStatus = "driver_unavailable"
    HIDDevicesUnavailable HIDStatus = "devices_unavailable"
)

// ProbeWarning represents risk flags derived from probe facts.
const (
    WarnUIPILikelyBlocked ProbeWarning = "uipi_likely_blocked"
    WarnIntegrityUnknown  ProbeWarning = "integrity_unknown"
    WarnUIAccessUnknown   ProbeWarning = "uiaccess_unknown"
)
```

---

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
SetHIDLibraryPath sets the custom path for `interception.dll`. It must be called before the first successful `ProbeHID`, `NewInput(BackendHID)`, or `SetBackend(BackendHID)`.

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

### func MouseDown

```go
func MouseDown(button MouseButton) error
```
MouseDown presses the specified button at the current cursor position without moving. Invalid button identities return `ErrInvalidMouseButton`.

### func MouseUp

```go
func MouseUp(button MouseButton) error
```
MouseUp releases the specified button at the current cursor position without moving. Invalid button identities return `ErrInvalidMouseButton`.

### func ClickMouse

```go
func ClickMouse() error
```
ClickMouse performs a left click at the current cursor position without moving.

### func MoveMouseBy

```go
func MoveMouseBy(dx, dy int32) error
```
MoveMouseBy moves the cursor by a delta. Under `BackendHID` this is one relative stroke. Under `BackendMessage` it reads the cursor, adds the delta using `int64`, and calls `SetCursorPos`.

### func MoveMouseToExact

```go
func MoveMouseToExact(x, y int32) error
```
MoveMouseToExact performs one absolute move. It rejects points outside the virtual desktop with `ErrOutsideVirtualDesktop`. Under HID it waits 12ms and verifies convergence within 1 pixel tolerance.

### func ScrollWheel

```go
func ScrollWheel(steps int) error
```
ScrollWheel scrolls by whole wheel steps. Positive steps scroll forward, negative backward. Zero and values outside `[-273, 273]` return `ErrInvalidScrollSteps`.

### func PressFor

```go
func PressFor(key Key, duration time.Duration) error
```
PressFor holds a global key for the requested duration. Durations below 10ms return `ErrInvalidPressDuration`.

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
Press simulates a global key press (down then up). The optional `interval` is the hold time (delay between a successful key-down and release, default 30ms).

### func PressHotkey

```go
func PressHotkey(keys ...Key) error
```
PressHotkey simulates a key combination (e.g., Ctrl+C). It presses keys in order, waits 50ms, then releases them in reverse order.

### func Type

```go
func Type(text string, interval ...time.Duration) error
```
Type simulates global text input by simulating keystrokes for each character. The optional `interval` is the character gap on both backends.

### func KeyFromRune

```go
func KeyFromRune(r rune) (Key, bool)
```
KeyFromRune attempts to map a unicode character (rune) to a Scan Code `Key`. Returns `(Key, true)` on success, or `(0, false)` if unsupported.

### func PasteByClipboard

```go
func PasteByClipboard(text string) error
```
PasteByClipboard pastes text into the current foreground window using Ctrl+V, following the global backend from `SetBackend`. It temporarily replaces the clipboard and restores `CF_UNICODETEXT` afterwards.

### func ProbeTarget

```go
func ProbeTarget(w *Window) (*TargetProbe, error)
```
ProbeTarget reports environmental facts about the target window without modifying global backend state or sending input. Reports process file name, target class, integrity relation, caller UIAccess, and derived UIPI warnings.

### func ProbeHID

```go
func ProbeHID() HIDStatus
```
ProbeHID reports whether this process can initialize the HID backend without changing the global backend. Returns `HIDInitialized`, `HIDDLLUnavailable`, `HIDDriverUnavailable`, `HIDDevicesUnavailable`, or `HIDUnknown`.

### func NewInput

```go
func NewInput(backend Backend) (*Input, error)
```
NewInput creates an input executor bound to a fixed backend chosen at construction. A later `SetBackend` does not change it. Rejects any value other than `BackendMessage` and `BackendHID` with `ErrInvalidBackend`.

### func FindByTitle

```go
func FindByTitle(title string) (*Window, error)
```
FindByTitle searches for a top-level window matching the exact title.

### func FindByClass

```go
func FindByClass(class string) (*Window, error)
```
FindByClass searches for a top-level window matching the class name.

### func FindByPID

```go
func FindByPID(pid uint32) ([]*Window, error)
```
FindByPID returns all top-level windows belonging to the specified Process ID.

### func FindByProcessName

```go
func FindByProcessName(name string) ([]*Window, error)
```
FindByProcessName returns every top-level window belonging to any process whose executable name matches.

---

## Types

### type Backend

```go
type Backend int
```
Backend represents the input simulation backend (`BackendMessage` or `BackendHID`).

### type MouseButton

```go
type MouseButton uint8
```
MouseButton identifies a mouse button (`MouseLeft`, `MouseRight`, `MouseMiddle`).

### type Key

```go
type Key = keyboard.Key
```
Key represents a composite keyboard scan code (Set 1).

### type Window

```go
type Window struct {
    HWND uintptr
}
```

#### func (*Window) FindChildByClass

```go
func (w *Window) FindChildByClass(class string) (*Window, error)
```
FindChildByClass searches for a direct child window with the specified class name (e.g., "Edit" inside Notepad).

#### func (*Window) IsValid

```go
func (w *Window) IsValid() bool
```
IsValid checks whether the window handle is a valid existing window.

#### func (*Window) IsVisible

```go
func (w *Window) IsVisible() bool
```
IsVisible checks whether the window is visible and not minimized.

#### func (*Window) Move

```go
func (w *Window) Move(x, y int32) error
```
Move moves the mouse cursor to coordinates relative to the window's client area.

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
Scroll performs a vertical mouse wheel scroll at the specified client coordinates. Under `BackendMessage` it posts `WM_MOUSEWHEEL`. Under `BackendHID` it returns `ErrDirectedScrollUnsupported`.

#### func (*Window) Text

```go
func (w *Window) Text() (string, error)
```
Text returns the current text of the target window/control using standard Win32 text retrieval (`WM_GETTEXT`).

#### func (*Window) Value

```go
func (w *Window) Value() (string, error)
```
Value returns the current best-effort textual value of the target window/control. It first tries `Text()`, then falls back to Windows UI Automation.

#### func (*Window) KeyDown

```go
func (w *Window) KeyDown(key Key) error
```
KeyDown sends a key down event to the window. Under `BackendHID`, the window's top-level ancestor must be foreground, otherwise returns `ErrTargetNotForeground`.

#### func (*Window) KeyUp

```go
func (w *Window) KeyUp(key Key) error
```
KeyUp sends a key up event to the window. Under `BackendHID`, releases are still sent even if not foreground.

#### func (*Window) Press

```go
func (w *Window) Press(key Key, interval ...time.Duration) error
```
Press simulates a full keystroke (KeyDown followed by KeyUp). Optional `interval` specifies hold time.

#### func (*Window) PressHotkey

```go
func (w *Window) PressHotkey(keys ...Key) error
```
PressHotkey presses keys in order and releases in reverse order. Modifier combinations under `BackendMessage` temporarily use foreground events for reliability.

#### func (*Window) Type

```go
func (w *Window) Type(text string, interval ...time.Duration) error
```
Type types text into the window, handling Shift modifiers. Optional `interval` sets character gap.

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

#### func (*Window) ClientInfo

```go
func (w *Window) ClientInfo() (x, y, width, height int32, err error)
```
ClientInfo returns the client-area rectangle in screen coordinates `(x, y, width, height)`.

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

---

### type Input

```go
type Input struct {
    // Unexported fields
}
```
Input is an input executor bound to one fixed backend chosen at construction time via `NewInput`.

#### func (*Input) ForWindow

```go
func (i *Input) ForWindow(w *Window) (*WindowInput, error)
```
ForWindow binds a window `w` to this executor, returning a `*WindowInput` facade.

#### func (*Input) Desktop

```go
func (i *Input) Desktop() (*DesktopInput, error)
```
Desktop returns the screen-coordinate and windowless `*DesktopInput` facade for this executor.

---

### type WindowInput

```go
type WindowInput struct {
    // Unexported fields
}
```
WindowInput is the window-targeted facade of an `Input`. It provides window-client operations bound to the fixed backend.

#### WindowInput Methods

```go
func (in *WindowInput) Move(x, y int32) error
func (in *WindowInput) MoveRel(dx, dy int32) error
func (in *WindowInput) Click(x, y int32) error
func (in *WindowInput) ClickRight(x, y int32) error
func (in *WindowInput) ClickMiddle(x, y int32) error
func (in *WindowInput) DoubleClick(x, y int32) error
func (in *WindowInput) Scroll(x, y, delta int32) error
func (in *WindowInput) KeyDown(key Key) error
func (in *WindowInput) KeyUp(key Key) error
func (in *WindowInput) Press(key Key, interval ...time.Duration) error
func (in *WindowInput) PressHotkey(keys ...Key) error
func (in *WindowInput) Type(text string, interval ...time.Duration) error
```

---

### type DesktopInput

```go
type DesktopInput struct {
    // Unexported fields
}
```
DesktopInput is the screen-coordinate and windowless facade of an `Input`. It provides virtual-desktop operations bound to the fixed backend.

#### DesktopInput Methods

```go
func (d *DesktopInput) MoveMouseTo(x, y int32) error
func (d *DesktopInput) ClickMouseAt(x, y int32) error
func (d *DesktopInput) DoubleClickMouseAt(x, y int32) error
func (d *DesktopInput) ClickRightMouseAt(x, y int32) error
func (d *DesktopInput) ClickMiddleMouseAt(x, y int32) error
func (d *DesktopInput) MouseDown(button MouseButton) error
func (d *DesktopInput) MouseUp(button MouseButton) error
func (d *DesktopInput) ClickMouse() error
func (d *DesktopInput) MoveMouseBy(dx, dy int32) error
func (d *DesktopInput) MoveMouseToExact(x, y int32) error
func (d *DesktopInput) ScrollWheel(steps int) error
func (d *DesktopInput) PressFor(key Key, duration time.Duration) error
func (d *DesktopInput) KeyDown(key Key) error
func (d *DesktopInput) KeyUp(key Key) error
func (d *DesktopInput) Press(key Key, interval ...time.Duration) error
func (d *DesktopInput) PressHotkey(keys ...Key) error
func (d *DesktopInput) Type(text string, interval ...time.Duration) error
```

#### func (*DesktopInput) PasteByClipboard

```go
func (d *DesktopInput) PasteByClipboard(text string) error
```
PasteByClipboard pastes text into the foreground window using Ctrl+V with the executor's fixed backend.

---

### type TargetProbe

```go
type TargetProbe struct {
    Features TargetFeatures
    Warnings []ProbeWarning
}
```

### type TargetFeatures

```go
type TargetFeatures struct {
    ProcessName       string
    TargetClass       string
    IntegrityRelation IntegrityRelation
    CallerUIAccess    UIAccess
}
```

### type IntegrityRelation

```go
type IntegrityRelation string
```

### type UIAccess

```go
type UIAccess string
```

### type HIDStatus

```go
type HIDStatus string
```

### type ProbeWarning

```go
type ProbeWarning string
```

---

## Screen Package (`github.com/rpdg/winput/screen`)

### type Point

```go
type Point struct {
    X int32
    Y int32
}
```

### type Rect

```go
type Rect struct {
    Left   int32
    Top    int32
    Right  int32
    Bottom int32
}
```

### type Monitor

```go
type Monitor struct {
    Handle     uintptr
    Bounds     Rect
    WorkArea   Rect // Excludes taskbar
    Primary    bool
    DeviceName string
}
```

### type CaptureOptions

```go
type CaptureOptions struct {
    PreserveAlpha bool // Deprecated: ignored, alpha is always 255
    MaxMemoryMB   int  // Peak budget in MiB (DIB + Go image). Default: 1000
}
```

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

### func ImageToVirtual

```go
func ImageToVirtual(imageX, imageY int32) (int32, int32)
```
ImageToVirtual converts coordinates from a full virtual desktop screenshot to actual Windows Virtual Desktop coordinates usable by `MoveMouseTo` and `ClickMouseAt`.

### func CaptureVirtualDesktop

```go
func CaptureVirtualDesktop() (*image.RGBA, error)
```
CaptureVirtualDesktop captures the entire virtual desktop (all monitors) using GDI `BitBlt`.

### func CaptureVirtualDesktopWithOptions

```go
func CaptureVirtualDesktopWithOptions(opts CaptureOptions) (*image.RGBA, error)
```
CaptureVirtualDesktopWithOptions captures the entire virtual desktop with explicit options.

### func CaptureRegion

```go
func CaptureRegion(x, y, w, h int32) (*image.RGBA, error)
```
CaptureRegion captures the intersection of the requested rectangle with the virtual desktop.

### func CaptureRegionWithOptions

```go
func CaptureRegionWithOptions(x, y, w, h int32, opts CaptureOptions) (*image.RGBA, error)
```
CaptureRegionWithOptions captures a specified region with explicit memory options.
