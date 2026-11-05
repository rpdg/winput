# winput API 参考手册

`winput` 包提供了一个用于 Windows 后台输入自动化的高级接口。

## 索引

*   [变量](#变量)
*   [常量](#常量)
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
*   [func ClickRightMouseAt](#func-clickrightmouseat)
*   [func ClickMiddleMouseAt](#func-clickmiddlemouseat)
*   [func DoubleClickMouseAt](#func-doubleclickmouseat)
*   [func KeyDown](#func-keydown)
*   [func KeyUp](#func-keyup)
*   [func Press](#func-press)
*   [func PressHotkey](#func-presshotkey)
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

## 变量

```go
var (
    ErrWindowNotFound     = errors.New("window not found")     // 未找到窗口
    ErrWindowGone         = errors.New("window is gone")       // 窗口句柄失效
    ErrWindowNotVisible   = errors.New("window is not visible")// 窗口不可见或最小化
    ErrUnsupportedKey     = errors.New("unsupported key")      // 不支持的按键
    ErrBackendUnavailable = errors.New("backend unavailable")  // 后端不可用
    ErrDriverNotInstalled = errors.New("driver not installed") // 驱动未安装 (仅 HID)
    ErrDLLLoadFailed      = errors.New("dll load failed")      // DLL 加载失败 (仅 HID)
    ErrPermissionDenied   = errors.New("permission denied")    // 权限不足
    ErrPostMessageFailed  = errors.New("PostMessageW failed")  // PostMessageW 调用失败
    ErrReadTextFailed     = errors.New("failed to read window text") // 读取文本失败
)
```

## Screen 包 (`github.com/rpdg/winput/screen`)

### func ImageToVirtual

```go
func ImageToVirtual(imageX, imageY int32) (int32, int32)
```
ImageToVirtual 将“完整虚拟桌面截图”中的坐标（OpenCV 匹配点）转换为 `MoveMouseTo` 可用的实际 Windows 虚拟桌面坐标。

### func VirtualBounds

```go
func VirtualBounds() Rect
```
VirtualBounds 返回整个虚拟桌面的边界矩形。

### func Monitors

```go
func Monitors() ([]Monitor, error)
```
Monitors 返回所有活动显示器及其几何信息的列表。

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

### 按键常量 (Key Constants)
常用键盘扫描码。

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
SetBackend 设置输入模拟后端。如果选择 `BackendHID`，它会立即尝试初始化驱动，如果失败（如驱动未安装）则返回错误。

### func SetHIDLibraryPath

```go
func SetHIDLibraryPath(path string)
```
SetHIDLibraryPath 设置 `interception.dll` 的自定义加载路径。

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

### func MouseDown

```go
func MouseDown(button MouseButton) error
```

MouseDown 在当前光标位置按下指定按钮，不移动光标。非法按钮返回 `ErrInvalidMouseButton`。输入互斥锁只覆盖这一次调用；按下、移动、抬起分成多次调用时不是原子操作。

### func MouseUp

```go
func MouseUp(button MouseButton) error
```

MouseUp 在当前光标位置抬起指定按钮，不移动光标。非法按钮返回 `ErrInvalidMouseButton`。输入互斥锁只覆盖这一次调用；按下、移动、抬起分成多次调用时不是原子操作。

### func ClickMouse

```go
func ClickMouse() error
```

ClickMouse 在当前光标位置执行左键单击，不移动光标。`BackendMessage` 下使用 `mouse_event`，该调用没有返回值，因此按钮事件的投递失败不可观测。

### func MoveMouseBy

```go
func MoveMouseBy(dx, dy int32) error
```

MoveMouseBy 按增量移动光标。`BackendHID` 发送一条相对笔画。`BackendMessage` 读取当前光标，用 `int64` 相加后调用 `SetCursorPos`；成功表示系统接受了新坐标。

### func MoveMouseToExact

```go
func MoveMouseToExact(x, y int32) error
```

MoveMouseToExact 执行一次绝对移动。虚拟桌面外的点返回 `ErrOutsideVirtualDesktop`。HID 路径发送一条虚拟桌面绝对笔画，等待 12ms，并按本进程坐标复核 1 像素；失败返回 `ErrMoveNotConverged`。这个 1 像素是本进程坐标空间里的读数，不是物理像素证明（RISK-001）。消息路径调用 `SetCursorPos`，成功表示系统接受了调用；它不使用这套归一化，也不等待。

### func ScrollWheel

```go
func ScrollWheel(steps int) error
```

ScrollWheel 按整格滚动。正数向前，负数向后。`0` 和 `[-273, 273]` 之外的值在乘法之前返回 `ErrInvalidScrollSteps`。消息后端使用 `mouse_event`，HID 后端发送一条滚轮笔画。两者都不移动光标。

### func PressFor

```go
func PressFor(key Key, duration time.Duration) error
```

PressFor 全局按住指定键，时长短于 10ms 返回 `ErrInvalidPressDuration`。消息后端使用扫描码 `SendInput`，HID 后端发送硬件按键。抬起失败会再试一次。

### func DoubleClickMouseAt

```go
func DoubleClickMouseAt(x, y int32) error
```
DoubleClickMouseAt 将鼠标移动到指定屏幕坐标并执行左键双击。

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
func Press(k Key) error
```
Press 模拟一次全局按键（按下后抬起），中间有短暂延迟。

### func PressHotkey

```go
func PressHotkey(keys ...Key) error
```
PressHotkey 模拟组合键（如 Ctrl+C）。它按顺序按下所有键，保持 50ms，然后按相反顺序释放。

### func Type

```go
func Type(text string) error
```
Type 模拟全局文本输入（通过模拟按键序列）。

### func CaptureVirtualDesktop

```go
func CaptureVirtualDesktop() (*image.RGBA, error)
```
CaptureVirtualDesktop (在 `screen` 包中) 使用 GDI 捕获整个虚拟桌面（所有显示器）。要求进程已开启 Per-Monitor DPI 感知。返回 `*image.RGBA` 对象。

### func CaptureRegion

```go
func CaptureRegion(x, y, w, h int32) (*image.RGBA, error)
```
CaptureRegion (在 `screen` 包中) 捕获虚拟桌面的指定区域。
`x`, `y` 为虚拟桌面坐标（允许负值）。
`w`, `h` 为像素尺寸。
内部调用 `CaptureVirtualDesktop`，转换坐标并执行安全裁剪。

## 类型

### type MouseButton

```go
type MouseButton uint8
```

MouseButton 表示鼠标按钮身份。公开值为 `MouseLeft`、`MouseRight`、`MouseMiddle`；驱动相关数值由库内部映射。

### type Window

#### func FindByTitle

```go
func FindByTitle(title string) (*Window, error)
```
FindByTitle 搜索精确匹配标题的顶级窗口。

#### func FindByClass

```go
func FindByClass(class string) (*Window, error)
```
FindByClass 搜索匹配类名的顶级窗口。

#### func FindByPID

```go
func FindByPID(pid uint32) ([]*Window, error)
```
FindByPID 返回属于指定进程 ID 的所有顶级窗口。

#### func FindByProcessName

```go
func FindByProcessName(name string) ([]*Window, error)
```
FindByProcessName 返回属于指定可执行文件名称的所有顶级窗口。

#### func (*Window) FindChildByClass

```go
func (w *Window) FindChildByClass(class string) (*Window, error)
```
FindChildByClass 搜索具有指定类名的子窗口（例如 Notepad 内部的 "Edit" 控件）。

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
Scroll 在指定客户区坐标执行鼠标滚轮滚动。

`BackendMessage` 会把 `WM_MOUSEWHEEL` 投递给目标窗口。`BackendHID` 返回 `ErrDirectedScrollUnsupported`，因为硬件滚轮事件无法定向到某个窗口；需要硬件级整格滚动时使用 `ScrollWheel`。

#### func (*Window) Text

```go
func (w *Window) Text() (string, error)
```
Text 使用标准 Win32 文本读取路径获取目标窗口/控件的当前文本。主要适用于 `Edit`、`RichEdit` 等标准 Win32 文本控件。

#### func (*Window) KeyDown

```go
func (w *Window) KeyDown(key Key) error
```
KeyDown 向窗口发送按键按下事件。

#### func (*Window) KeyUp

```go
func (w *Window) KeyUp(key Key) error
```
KeyUp 向窗口发送按键抬起事件。

#### func (*Window) Press

```go
func (w *Window) Press(key Key) error
```
Press 模拟一次完整的按键过程。

#### func (*Window) PressHotkey

```go
func (w *Window) PressHotkey(keys ...Key) error
```
PressHotkey 执行组合键（如 Ctrl+A）。
当它作用于窗口且使用 `BackendMessage` 时，包含 `Ctrl`、`Alt`、`Shift`
等修饰键的组合会回退为前台键盘事件发送，而不是纯 `PostMessage`。
原因是很多应用仅靠窗口消息无法可靠识别修饰键状态。

#### func (*Window) Type

```go
func (w *Window) Type(text string) error
```
输入字符串，自动处理大写字母和符号的 Shift 切换。

#### func (*Window) Value

```go
func (w *Window) Value() (string, error)
```
Value 返回目标窗口/控件当前的“最佳努力”文本值。它会先尝试 `Text()` 所使用的 Win32 读取路径，必要时再回退到 Windows UI Automation 读取现代控件。

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
