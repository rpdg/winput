# winput API 参考手册

`winput` 包提供了一个用于 Windows 后台输入自动化的高级接口。

## 索引

*   [变量](#变量)
*   [常量](#常量)
    *   [后端常量 (Backend Constants)](#后端常量-backend-constants)
    *   [鼠标按键常量 (MouseButton Constants)](#鼠标按键常量-mousebutton-constants)
    *   [按键常量 (Key Constants)](#按键常量-key-constants)
    *   [探针常量 (Probe Constants)](#探针常量-probe-constants)
*   [函数](#函数)
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
*   [类型](#类型)
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
        *   [WindowInput 方法列表](#windowinput-方法列表)
    *   [type DesktopInput](#type-desktopinput)
        *   [DesktopInput 方法列表](#desktopinput-方法列表)
        *   [func (*DesktopInput) PasteByClipboard](#func-desktopinput-pastebyclipboard)
    *   [type TargetProbe](#type-targetprobe)
    *   [type TargetFeatures](#type-targetfeatures)
    *   [type IntegrityRelation](#type-integrityrelation)
    *   [type UIAccess](#type-uiaccess)
    *   [type HIDStatus](#type-hidstatus)
    *   [type ProbeWarning](#type-probewarning)
*   [Screen 包 (`github.com/rpdg/winput/screen`)](#screen-包-githubcomrpdgwinputscreen)
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

## 变量

```go
var (
    // ErrWindowNotFound 表示通过 Title、Class 或 PID 无法定位目标窗口。
    ErrWindowNotFound = errors.New("window not found")

    // ErrInvalidLookupString 表示窗口标题或类名包含嵌入的 NUL 字符。
    // 它同时被 ErrWindowNotFound 包裹，errors.Is 对两者均匹配。
    ErrInvalidLookupString = window.ErrInvalidLookupString

    // ErrWindowGone 表示窗口句柄失效或已销毁。
    ErrWindowGone = errors.New("window is gone or invalid")

    // ErrTargetChanged 表示探针期间窗口句柄有效，但其进程、线程或起始类名发生了改变。
    ErrTargetChanged = errors.New("target window changed during probe")

    // ErrInputInvalid 表示执行器或 Facade 并非由 NewInput、ForWindow 或 Desktop 产生。
    ErrInputInvalid = errors.New("input executor is invalid")

    // ErrInvalidBackend 表示向 NewInput 传入了 BackendMessage 和 BackendHID 以外的非法值。
    ErrInvalidBackend = errors.New("invalid input backend")

    // ErrWindowNotVisible 表示窗口不可见或处于最小化状态。
    ErrWindowNotVisible = errors.New("window is not visible")

    // ErrUnsupportedKey 是 keyboard.ErrUnsupportedKey 的别名，文本为 "unsupported key or character"。
    ErrUnsupportedKey = keyboard.ErrUnsupportedKey

    // ErrBackendUnavailable 表示选定的输入后端初始化失败。
    ErrBackendUnavailable = errors.New("input backend unavailable")

    // ErrDriverNotInstalled 专属于 BackendHID，表示 Interception 驱动未安装或无法打开设备。
    ErrDriverNotInstalled = errors.New("interception driver not installed or accessible")

    // ErrDLLLoadFailed 表示无法加载 interception.dll 或缺少导出符号。
    ErrDLLLoadFailed = errors.New("failed to load interception library")

    // ErrPermissionDenied 表示由于系统权限限制（如 UIPI）导致操作被拒绝。
    ErrPermissionDenied = errors.New("permission denied")

    // ErrPostMessageFailed 是 window.ErrPostMessageFailed 的别名，表示 PostMessageW 返回 0。
    ErrPostMessageFailed = window.ErrPostMessageFailed

    // ErrReadTextFailed 是 window.ErrReadTextFailed 的别名，表示读取窗口文本失败。
    ErrReadTextFailed = window.ErrReadTextFailed

    // ErrMoveNotConverged 是 hid.ErrMoveNotConverged 的别名，表示 HID 移动在容差内未收敛。
    ErrMoveNotConverged = hid.ErrMoveNotConverged

    // ErrOutsideVirtualDesktop 是 hid.ErrOutsideVirtualDesktop 的别名，表示坐标落在虚拟桌面之外。
    ErrOutsideVirtualDesktop = hid.ErrOutsideVirtualDesktop

    // ErrSendInputFailed 表示 SendInput 写入了 0 个事件。
    ErrSendInputFailed = errors.New("SendInput inserted no events")

    // ErrSendInputPartial 表示 SendInput 仅写入了按下事件，且补偿抬起成功。
    ErrSendInputPartial = errors.New("SendInput partially inserted and was repaired")

    // ErrSendInputStuck 表示 SendInput 仅写入了按下事件，且补偿抬起失败。
    ErrSendInputStuck = errors.New("SendInput partially inserted and could not be repaired")

    // ErrDirectedScrollUnsupported 表示 HID 后端不支持向特定窗口投递定向滚轮事件。
    ErrDirectedScrollUnsupported = errors.New("HID backend does not support directed scrolling")

    // ErrTargetNotForeground 表示在 BackendHID 下，目标窗口的顶层祖先不是当前前台窗口。
    ErrTargetNotForeground = errors.New("target window is not the foreground window")

    // ErrInvalidMouseButton 表示 MouseButton 数值非法。
    ErrInvalidMouseButton = errors.New("invalid mouse button")

    // ErrInvalidMouseCoordinate 表示鼠标消息坐标超出 signed 16-bit 范围。
    ErrInvalidMouseCoordinate = mouse.ErrInvalidMouseCoordinate

    // ErrInvalidScrollDelta 是 mouse.ErrInvalidScrollDelta 的别名，表示滚轮增量超出 int16 范围。
    ErrInvalidScrollDelta = mouse.ErrInvalidScrollDelta

    // ErrInvalidScrollSteps 表示 ScrollWheel 步数为 0 或超出 [-273, 273] 范围。
    ErrInvalidScrollSteps = errors.New("scroll steps must be non-zero and within [-273, 273]")

    // ErrInvalidPressDuration 表示 PressFor 给定时长小于 10ms。
    ErrInvalidPressDuration = errors.New("press duration must be at least 10ms")

    // ErrInvalidInterval 是 keyboard.ErrInvalidInterval 的别名。
    // Press / Type 的可选间隔为负数或传入多于一个参数时返回它（间隔为 0 是合法的）。
    ErrInvalidInterval = keyboard.ErrInvalidInterval

    // ErrEmptyPasteText 表示用空字符串调用 PasteByClipboard。剪贴板不会被打开，也不会发送按键。
    ErrEmptyPasteText = errors.New("paste text is empty")

    // ErrClipboardFailed 是 window.ErrClipboardFailed 的别名，表示剪贴板打开、读写或恢复失败。
    ErrClipboardFailed = window.ErrClipboardFailed
)
```

---

## 常量

### 后端常量 (Backend Constants)

```go
const (
    // BackendMessage 使用标准的 Windows 消息 (PostMessage) 进行输入。
    BackendMessage Backend = iota

    // BackendHID 使用 Interception 驱动程序模拟硬件输入。
    BackendHID
)
```

### 鼠标按键常量 (MouseButton Constants)

```go
const (
    MouseLeft MouseButton = iota + 1
    MouseRight
    MouseMiddle
)
```

### 按键常量 (Key Constants)

`Key` 是复合值。低 8 位是 Set 1 扫描码，需要 E0 前缀的键把高字节设为 `0xE0`。不要把整个值当成扫描码：`0x48` 是小键盘 8（`KeyNumpad8`），方向键上是 `0xE048`（`KeyArrowUp`）。

```go
const (
    KeyEsc, KeyEnter, KeySpace, KeyTab, KeyBkSp Key = ...
    KeyShift, KeyCtrl, KeyAlt, KeyCaps          Key = ...
    KeyLeftCtrl                                 Key = 0x1D
    KeyRightCtrl                                Key = 0xE01D
    KeyLeftAlt                                  Key = 0x38
    KeyRightAlt                                 Key = 0xE038
    KeyNumpadDivide, KeyDivide                  Key = 0xE035 // KeyDivide 是 KeyNumpadDivide 的别名
    KeyLWin                                     Key = 0xE05B
    KeyRWin                                     Key = 0xE05C
    KeyF1 .. KeyF12                             Key = ...
    KeyA .. KeyZ                                Key = ...
    Key0 .. Key9                                Key = ...
    KeyArrowUp, KeyArrowDown, KeyLeft, KeyRight Key = ... // 高字节为 0xE0
    KeyHome, KeyEnd, KeyPageUp, KeyPageDown     Key = ...
    KeyInsert, KeyDelete                        Key = ...
)
```

### 探针常量 (Probe Constants)

```go
// IntegrityRelation 表示从调用方视角的进程完整性级别关系。
const (
    IntegrityUnknown      IntegrityRelation = "unknown"
    IntegrityTargetHigher IntegrityRelation = "target_higher"
    IntegrityEqual        IntegrityRelation = "equal"
    IntegrityTargetLower  IntegrityRelation = "target_lower"
)

// UIAccess 表示调用方令牌的 UIAccess 标志。
const (
    UIAccessUnknown  UIAccess = "unknown"
    UIAccessEnabled  UIAccess = "enabled"
    UIAccessDisabled UIAccess = "disabled"
)

// HIDStatus 表示本进程 HID 后端的初始化结果。
const (
    HIDUnknown            HIDStatus = "unknown"
    HIDInitialized        HIDStatus = "initialized"
    HIDDLLUnavailable     HIDStatus = "dll_unavailable"
    HIDDriverUnavailable  HIDStatus = "driver_unavailable"
    HIDDevicesUnavailable HIDStatus = "devices_unavailable"
)

// ProbeWarning 表示由探针事实导出的输入风险提示。
const (
    WarnUIPILikelyBlocked ProbeWarning = "uipi_likely_blocked"
    WarnIntegrityUnknown  ProbeWarning = "integrity_unknown"
    WarnUIAccessUnknown   ProbeWarning = "uiaccess_unknown"
)
```

---

## 函数

### func EnablePerMonitorDPI

```go
func EnablePerMonitorDPI() error
```
EnablePerMonitorDPI 将当前进程设置为 Per-Monitor (v2) DPI 感知。

### func GetCursorPos

```go
func GetCursorPos() (int32, int32, error)
```
GetCursorPos 返回鼠标光标当前的绝对屏幕坐标。

### func SetBackend

```go
func SetBackend(b Backend) error
```
SetBackend 设置全局输入模拟后端。如果选择 `BackendHID`，会立即尝试初始化驱动，若失败则返回错误。

### func SetHIDLibraryPath

```go
func SetHIDLibraryPath(path string)
```
SetHIDLibraryPath 设置 `interception.dll` 的自定义加载路径。必须在首次成功的 `ProbeHID`、`NewInput(BackendHID)` 或 `SetBackend(BackendHID)` 之前调用。

### func MoveMouseTo

```go
func MoveMouseTo(x, y int32) error
```
MoveMouseTo 将鼠标光标移动到绝对屏幕坐标（虚拟桌面）。

### func ClickMouseAt

```go
func ClickMouseAt(x, y int32) error
```
ClickMouseAt 将鼠标移动到指定屏幕坐标并执行左键点击。

### func ClickRightMouseAt

```go
func ClickRightMouseAt(x, y int32) error
```
ClickRightMouseAt 将鼠标移动到指定屏幕坐标并执行右键点击。

### func ClickMiddleMouseAt

```go
func ClickMiddleMouseAt(x, y int32) error
```
ClickMiddleMouseAt 将鼠标移动到指定屏幕坐标并执行中键点击。

### func DoubleClickMouseAt

```go
func DoubleClickMouseAt(x, y int32) error
```
DoubleClickMouseAt 将鼠标移动到指定屏幕坐标并执行左键双击。

### func MouseDown

```go
func MouseDown(button MouseButton) error
```
MouseDown 在当前光标位置按下指定按钮，不移动光标。非法按钮返回 `ErrInvalidMouseButton`。

### func MouseUp

```go
func MouseUp(button MouseButton) error
```
MouseUp 在当前光标位置抬起指定按钮，不移动光标。非法按钮返回 `ErrInvalidMouseButton`。

### func ClickMouse

```go
func ClickMouse() error
```
ClickMouse 在当前光标位置执行左键单击，不移动光标。

### func MoveMouseBy

```go
func MoveMouseBy(dx, dy int32) error
```
MoveMouseBy 按增量移动光标。`BackendHID` 发送相对笔画；`BackendMessage` 读取当前光标后调用 `SetCursorPos`。

### func MoveMouseToExact

```go
func MoveMouseToExact(x, y int32) error
```
MoveMouseToExact 执行一次绝对移动。超出虚拟桌面的坐标返回 `ErrOutsideVirtualDesktop`。HID 路径会验证 1 像素容差内的收敛状态。

### func ScrollWheel

```go
func ScrollWheel(steps int) error
```
ScrollWheel 按整格滚动滚轮。正数向前，负数向后。步数为 0 或超出 `[-273, 273]` 返回 `ErrInvalidScrollSteps`。

### func PressFor

```go
func PressFor(key Key, duration time.Duration) error
```
PressFor 全局按住指定键，时长短于 10ms 返回 `ErrInvalidPressDuration`。

### func KeyDown

```go
func KeyDown(k Key) error
```
KeyDown 模拟全局按键按下事件。

### func KeyUp

```go
func KeyUp(k Key) error
```
KeyUp 模拟全局按键抬起事件。

### func Press

```go
func Press(k Key, interval ...time.Duration) error
```
Press 模拟全局按键（按下后抬起）。可选 `interval` 指定按下保持时间（默认 30ms）。

### func PressHotkey

```go
func PressHotkey(keys ...Key) error
```
PressHotkey 模拟组合键（如 Ctrl+C）。按顺序按下各键，保持 50ms 后按倒序释放。

### func Type

```go
func Type(text string, interval ...time.Duration) error
```
Type 模拟全局文本输入。可选 `interval` 为字符间隙。

### func KeyFromRune

```go
func KeyFromRune(r rune) (Key, bool)
```
KeyFromRune 尝试将 unicode 字符（rune）映射为扫描码 `Key`。映射成功返回 `(Key, true)`，不支持返回 `(0, false)`。

### func PasteByClipboard

```go
func PasteByClipboard(text string) error
```
PasteByClipboard 通过 Ctrl+V 把文本粘贴到当前前台窗口，跟随 `SetBackend` 的全局后端。临时替换剪贴板，并在发送组合键后恢复 `CF_UNICODETEXT`。

### func ProbeTarget

```go
func ProbeTarget(w *Window) (*TargetProbe, error)
```
ProbeTarget 报告目标窗口的环境事实，不修改全局后端，不发送输入消息。报告进程文件名、类名、完整性关系、调用方 UIAccess 及导出的 UIPI 风险警告。

### func ProbeHID

```go
func ProbeHID() HIDStatus
```
ProbeHID 报告本进程能否初始化 HID 后端，不修改全局后端。返回 `HIDInitialized`, `HIDDLLUnavailable`, `HIDDriverUnavailable`, `HIDDevicesUnavailable`, 或 `HIDUnknown`。

### func NewInput

```go
func NewInput(backend Backend) (*Input, error)
```
NewInput 创建绑定到指定后端的任务级输入执行器。其绑定的后端不可变，之后的 `SetBackend` 不影响该执行器。只接受 `BackendMessage` 与 `BackendHID`。

### func FindByTitle

```go
func FindByTitle(title string) (*Window, error)
```
FindByTitle 搜索精确匹配标题的顶级窗口。

### func FindByClass

```go
func FindByClass(class string) (*Window, error)
```
FindByClass 搜索匹配类名的顶级窗口。

### func FindByPID

```go
func FindByPID(pid uint32) ([]*Window, error)
```
FindByPID 返回属于指定进程 ID 的所有顶级窗口。

### func FindByProcessName

```go
func FindByProcessName(name string) ([]*Window, error)
```
FindByProcessName 返回属于同名可执行文件进程的全部顶级窗口。

---

## 类型

### type Backend

```go
type Backend int
```
Backend 表示输入模拟后端类型（`BackendMessage` 或 `BackendHID`）。

### type MouseButton

```go
type MouseButton uint8
```
MouseButton 表示鼠标按键（`MouseLeft`、`MouseRight`、`MouseMiddle`）。

### type Key

```go
type Key = keyboard.Key
```
Key 表示键盘 Set 1 扫描码复合值。

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
FindChildByClass 搜索具有指定类名的直接子窗口（例如记事本内的 "Edit" 控件）。

#### func (*Window) IsValid

```go
func (w *Window) IsValid() bool
```
IsValid 检查窗口句柄当前是否仍然有效。

#### func (*Window) IsVisible

```go
func (w *Window) IsVisible() bool
```
IsVisible 检查窗口是否可见且未处于最小化状态。

#### func (*Window) Move

```go
func (w *Window) Move(x, y int32) error
```
Move 将鼠标光标移动到相对于窗口客户区的指定坐标。

#### func (*Window) MoveRel

```go
func (w *Window) MoveRel(dx, dy int32) error
```
MoveRel 相对于当前鼠标位置移动光标。

#### func (*Window) Click

```go
func (w *Window) Click(x, y int32) error
```
Click 在指定的客户区坐标执行鼠标左键点击。

#### func (*Window) ClickRight

```go
func (w *Window) ClickRight(x, y int32) error
```
ClickRight 在指定的客户区坐标执行鼠标右键点击。

#### func (*Window) ClickMiddle

```go
func (w *Window) ClickMiddle(x, y int32) error
```
ClickMiddle 在指定的客户区坐标执行鼠标中键点击。

#### func (*Window) DoubleClick

```go
func (w *Window) DoubleClick(x, y int32) error
```
DoubleClick 执行鼠标左键双击。

#### func (*Window) Scroll

```go
func (w *Window) Scroll(x, y int32, delta int32) error
```
Scroll 在指定客户区坐标执行鼠标滚轮滚动。`BackendMessage` 下投递 `WM_MOUSEWHEEL`；`BackendHID` 下返回 `ErrDirectedScrollUnsupported`。

#### func (*Window) Text

```go
func (w *Window) Text() (string, error)
```
Text 使用标准 Win32 `WM_GETTEXT` 路径读取目标控件文本（主要适用于 Edit、RichEdit 等控件）。

#### func (*Window) Value

```go
func (w *Window) Value() (string, error)
```
Value 获取控件文本的最佳努力值。先尝试 `Text()`，必要时自动回退到 UI Automation。

#### func (*Window) KeyDown

```go
func (w *Window) KeyDown(key Key) error
```
KeyDown 向窗口发送按键按下事件。`BackendHID` 要求顶层祖先窗口必须处于前台，否则返回 `ErrTargetNotForeground`。

#### func (*Window) KeyUp

```go
func (w *Window) KeyUp(key Key) error
```
KeyUp 向窗口发送按键抬起事件。`BackendHID` 在非前台时仍发送抬起，以避免按键卡死。

#### func (*Window) Press

```go
func (w *Window) Press(key Key, interval ...time.Duration) error
```
Press 模拟完整击键过程（按下后抬起）。可选 `interval` 指定按住时间。

#### func (*Window) PressHotkey

```go
func (w *Window) PressHotkey(keys ...Key) error
```
PressHotkey 执行组合键。`BackendMessage` 下包含修饰键时临时使用前台键盘事件以确保可靠识别。

#### func (*Window) Type

```go
func (w *Window) Type(text string, interval ...time.Duration) error
```
Type 向窗口输入文本，自动处理 Shift 切换。

#### func (*Window) DPI

```go
func (w *Window) DPI() (uint32, uint32, error)
```
DPI 返回窗口的水平和垂直 DPI。

#### func (*Window) ClientRect

```go
func (w *Window) ClientRect() (width, height int32, err error)
```
ClientRect 返回窗口客户区的宽度和高度。

#### func (*Window) ClientInfo

```go
func (w *Window) ClientInfo() (x, y, width, height int32, err error)
```
ClientInfo 以屏幕坐标返回客户区矩形 `(x, y, width, height)`。

#### func (*Window) ScreenToClient

```go
func (w *Window) ScreenToClient(x, y int32) (cx, cy int32, err error)
```
ScreenToClient 将屏幕相对坐标转换为窗口客户区相对坐标。

#### func (*Window) ClientToScreen

```go
func (w *Window) ClientToScreen(x, y int32) (sx, sy int32, err error)
```
ClientToScreen 将窗口客户区相对坐标转换为屏幕相对坐标。

---

### type Input

```go
type Input struct {
    // 内部未导出字段
}
```
Input 是在构造阶段通过 `NewInput` 绑定了固定后端的输入执行器。

#### func (*Input) ForWindow

```go
func (i *Input) ForWindow(w *Window) (*WindowInput, error)
```
ForWindow 将窗口 `w` 绑定到该执行器，返回 `*WindowInput` 操作面。

#### func (*Input) Desktop

```go
func (i *Input) Desktop() (*DesktopInput, error)
```
Desktop 返回该执行器的屏幕绝对坐标与无窗口 `*DesktopInput` 操作面。

---

### type WindowInput

```go
type WindowInput struct {
    // 内部未导出字段
}
```
WindowInput 是 `Input` 的窗口专属操作 Facade，所有操作均使用创建时绑定的固定后端。

#### WindowInput 方法列表

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
    // 内部未导出字段
}
```
DesktopInput 是 `Input` 的屏幕坐标与无窗口专属 Facade，所有操作均使用创建时绑定的固定后端。

#### DesktopInput 方法列表

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
PasteByClipboard 使用该执行器绑定的固定后端向当前前台窗口发送 Ctrl+V 粘贴文本。

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

## Screen 包 (`github.com/rpdg/winput/screen`)

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
    WorkArea   Rect // 不包含任务栏
    Primary    bool
    DeviceName string
}
```

### type CaptureOptions

```go
type CaptureOptions struct {
    PreserveAlpha bool // 已弃用，恒为 255
    MaxMemoryMB   int  // 峰值内存预算（MiB，DIB + Go 图像）。默认：1000
}
```

### func VirtualBounds

```go
func VirtualBounds() Rect
```
VirtualBounds 返回整个虚拟桌面的边界矩形。

### func Monitors

```go
func Monitors() ([]Monitor, error)
```
Monitors 返回所有活动显示器及其几何信息。

### func ImageToVirtual

```go
func ImageToVirtual(imageX, imageY int32) (int32, int32)
```
ImageToVirtual 将全虚拟桌面截图中的像素坐标转换为 `MoveMouseTo` 可用的虚拟桌面全局坐标。

### func CaptureVirtualDesktop

```go
func CaptureVirtualDesktop() (*image.RGBA, error)
```
CaptureVirtualDesktop 使用 GDI `BitBlt` 截取整个虚拟桌面（包含所有显示器）。

### func CaptureVirtualDesktopWithOptions

```go
func CaptureVirtualDesktopWithOptions(opts CaptureOptions) (*image.RGBA, error)
```
CaptureVirtualDesktopWithOptions 使用指定选项截取整个虚拟桌面。

### func CaptureRegion

```go
func CaptureRegion(x, y, w, h int32) (*image.RGBA, error)
```
CaptureRegion 截取请求矩形与虚拟桌面的交集区域。

### func CaptureRegionWithOptions

```go
func CaptureRegionWithOptions(x, y, w, h int32, opts CaptureOptions) (*image.RGBA, error)
```
CaptureRegionWithOptions 使用指定内存选项截取目标区域。
