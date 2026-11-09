# winput API 参考手册

`winput` 包提供了一个用于 Windows 后台输入自动化的高级接口。

## 索引

*   [变量](#变量)
*   [常量](#常量)
*   [func EnablePerMonitorDPI](#func-enablepermonitordpi)
*   [func GetCursorPos](#func-getcursorpos)
*   [func SetBackend](#func-setbackend)
*   [func SetHIDLibraryPath](#func-sethidlibrarypath)
*   [func ProbeTarget](#func-probetarget)
*   [func ProbeHID](#func-probehid)
*   [func NewInput](#func-newinput)
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
*   [func PasteByClipboard](#func-pastebyclipboard)
*   [func (*DesktopInput) PasteByClipboard](#func-desktopinput-pastebyclipboard)
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

## 变量

```go
var (
    ErrWindowNotFound     = errors.New("window not found")     // 未找到窗口
    ErrWindowGone         = errors.New("window is gone or invalid") // 窗口句柄失效
    ErrWindowNotVisible   = errors.New("window is not visible")// 窗口不可见或最小化
    // 别名 keyboard.ErrUnsupportedKey，文本是 "unsupported key or character"，不是独立哨兵。
    ErrUnsupportedKey     = keyboard.ErrUnsupportedKey
    ErrBackendUnavailable = errors.New("input backend unavailable") // 后端不可用
    ErrDriverNotInstalled = errors.New("interception driver not installed or accessible") // 驱动未安装 (仅 HID)
    ErrDLLLoadFailed      = errors.New("failed to load interception library") // DLL 加载失败 (仅 HID)
    ErrPermissionDenied   = errors.New("permission denied")    // 权限不足
    ErrPostMessageFailed  = window.ErrPostMessageFailed        // PostMessageW 调用失败
    ErrReadTextFailed     = window.ErrReadTextFailed           // 读取文本失败
    ErrMoveNotConverged   = hid.ErrMoveNotConverged            // HID 移动未收敛
    ErrOutsideVirtualDesktop = hid.ErrOutsideVirtualDesktop    // 目标在虚拟桌面外
    ErrSendInputFailed    = errors.New("SendInput inserted no events") // SendInput 一条都没写入
    ErrSendInputPartial   = errors.New("SendInput partially inserted and was repaired") // 只写入按下，补抬起成功
    ErrSendInputStuck     = errors.New("SendInput partially inserted and could not be repaired") // 只写入按下，补抬起失败
    ErrDirectedScrollUnsupported = errors.New("HID backend does not support directed scrolling") // HID 不能把滚轮定向到窗口
    ErrInvalidMouseButton = errors.New("invalid mouse button") // 按钮身份非法
    // 别名 mouse.ErrInvalidScrollDelta。消息路径的 Window.Scroll 返回它。hid.ErrInvalidScrollDelta 是另一个哨兵。
    ErrInvalidScrollDelta = mouse.ErrInvalidScrollDelta        // 滚轮增量超出 int16
    ErrInvalidScrollSteps = errors.New("scroll steps must be non-zero and within [-273, 273]") // ScrollWheel 步数非法
    ErrInvalidPressDuration = errors.New("press duration must be at least 10ms") // PressFor 时长短于 10ms
    // 别名 keyboard.ErrInvalidInterval（"interval must be a single non-negative duration"）。
    // Press / Type 的可选间隔为负数或参数多于一个时返回它。它与 ErrInvalidPressDuration 有意区分：间隔 0 是合法的。
    ErrInvalidInterval     = keyboard.ErrInvalidInterval
    // ErrEmptyPasteText 表示用空字符串调用 PasteByClipboard。剪贴板不会被打开，也不会发送按键。
    ErrEmptyPasteText      = errors.New("paste text is empty")
    // 别名 window.ErrClipboardFailed。表示剪贴板打开、读取、写入或恢复失败。错误文本从不包含剪贴板内容。
    ErrClipboardFailed     = window.ErrClipboardFailed
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

ClickMouse 在当前光标位置执行左键单击，不移动光标。按钮标志与 `MouseDown` / `MouseUp` 共用同一映射。`BackendMessage` 下使用 `mouse_event`，该调用没有返回值，因此按钮事件的投递失败不可观测。`BackendHID` 下抬起失败会再试一次，两次都失败时保留第一次的错误。

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
func Press(k Key, interval ...time.Duration) error
```
Press 模拟一次全局按键（按下后抬起）。可选的 `interval` 是按住时间（按下成功后到抬起之间的延迟），两条后端都生效；不传时按住 30ms。`Press(key, 0)` 在按下与抬起之间不睡眠。间隔为负数或参数多于一个时，在任何按键之前返回 `ErrInvalidInterval`。

### func PressHotkey

```go
func PressHotkey(keys ...Key) error
```
PressHotkey 模拟组合键（如 Ctrl+C）。它按顺序按下所有键，保持 50ms，然后按相反顺序释放。

### func Type

```go
func Type(text string, interval ...time.Duration) error
```
Type 模拟全局文本输入（通过模拟按键序列）。可选的 `interval` 在两条后端都是字符间隙。消息后端在连续 rune 之间睡眠该间隙（默认 30ms），最后一个 rune 之后不再多睡。HID 每个字符的固定按住不变，只有传入显式间隔时才会在字符松开后再睡一段；不传参数则不增加额外间隙。间隔为负数或参数多于一个时，在任何字符发送之前返回 `ErrInvalidInterval`。

### func PasteByClipboard

```go
func PasteByClipboard(text string) error
```
PasteByClipboard 通过 Ctrl+V 把文本粘贴到当前前台窗口。它不接收 `HWND`：按键进入调用发生时已经是前台的窗口。

返回 nil **并不表示**前台程序已经收到新文本。nil 只表示 Ctrl+V 事件已经入队，并且先前剪贴板上的 Unicode 文本已经恢复且没有错误。`SendInput` 在事件入队时返回，并不等待前台处理粘贴，因此在恢复之前不存在「前台已经读完剪贴板」的同步点。返回非 nil 也不表示按键没有插入：按键可能已经成功而恢复失败，此时 `errors.Is` 只能认出 `ErrClipboardFailed`，不会匹配 `ErrSendInputFailed`；只有按键与恢复均失败时，同一个错误才会同时匹配两个哨兵。要确认控件是否收下文字，必须另行观察那个控件。

剪贴板会被临时替换，结束后只放回 `CF_UNICODETEXT`。系统可以把 `CF_TEXT` 作为 `CF_UNICODETEXT` 提供；此时读到的合成 Unicode 文本会以 `CF_UNICODETEXT` 恢复。若没有可读的 Unicode 文本（例如剪贴板上只有图片，或只有系统无法转换成 `CF_UNICODETEXT` 的格式），恢复的是空 Unicode 缓冲。两种情况下，原有的 `CF_TEXT` 及其它非文本格式都不会回来。新文本中单独的 `\n` 会先被规范成 `\r\n` 再写入；恢复的文本按读到的内容原样写回。空文本在打开剪贴板、发送任何按键之前以 `ErrEmptyPasteText` 失败。PasteByClipboard 不是 `Type` 的替代品；拒绝粘贴的控件（例如密码框）不会因此成功。

### func (*DesktopInput) PasteByClipboard

```go
func (d *DesktopInput) PasteByClipboard(text string) error
```
PasteByClipboard 通过 Ctrl+V 把文本粘贴到当前前台窗口，并使用该执行器在 `NewInput` 时固定的后端；调用时不会重新选择后端，之后调用 `SetBackend` 也不影响它。目标窗口仍是调用发生时处于前台的窗口，不接收 `HWND`。剪贴板属于当前窗口站，同一窗口站内的其它进程共享同一份内容；包级函数也使用这份剪贴板。

nil 或零值 `DesktopInput` 会在文本校验、后端解析和任何剪贴板访问之前返回 `ErrInputInvalid`。合法门面上的空串返回 `ErrEmptyPasteText`，不获取输入锁、不打开剪贴板；纯空白文本合法。

固定后端路径不会额外执行包装层可用性检查，也不会在解析后端时初始化 HID；构造阶段的 HID 初始化仍只发生在 `NewInput(BackendHID)`，底层运行期原有行为不变。因此运行期后端错误可能发生在剪贴板替换之后，此时仍会尝试恢复。剪贴板格式、恢复、`inputMutex`、前台窗口和消费时序的限制与包级 `PasteByClipboard` 相同：只恢复 `CF_UNICODETEXT`，返回 nil 不证明目标已消费文本，返回非 nil 也不证明没有发送按键。该方法不提供前台窗口隔离或跨进程剪贴板独占。


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
FindByProcessName 返回每一个同名可执行文件进程的全部顶级窗口。没有顶级窗口的进程会被跳过。顺序与进程快照一致。

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

`BackendMessage` 会把 `WM_MOUSEWHEEL` 投递给目标窗口。`delta == 0` 是空操作，其余增量必须落在 `int16` 内，否则返回 `ErrInvalidScrollDelta`（`mouse.ErrInvalidScrollDelta` 的别名）。`BackendHID` 不校验 `delta`，始终返回 `ErrDirectedScrollUnsupported`，因为硬件滚轮事件无法定向到某个窗口。需要硬件级整格滚动时使用 `ScrollWheel`。

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
func (w *Window) Press(key Key, interval ...time.Duration) error
```
Press 模拟一次完整的按键过程。可选的 `interval` 在两条后端都是按住时间（按下成功后到抬起之间的延迟）；不传时按住 30ms。`Press(key, 0)` 不睡眠。间隔为负数或参数多于一个时，在任何按键之前返回 `ErrInvalidInterval`。

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
func (w *Window) Type(text string, interval ...time.Duration) error
```
输入字符串，自动处理大写字母和符号的 Shift 切换。可选的 `interval` 是字符间隙：消息后端在连续 rune 之间睡眠该间隙（默认 30ms，内部走 `keyboard.TypeWithInterval`），最后一个 rune 之后不再多睡；HID 每个字符的固定按住不变，只有传入显式间隔时才会在字符松开后再睡一段。间隔为负数或参数多于一个时，在任何字符发送之前返回 `ErrInvalidInterval`。

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

#### func (*Window) ClientInfo

```go
func (w *Window) ClientInfo() (x, y, width, height int32, err error)
```
ClientInfo 以屏幕坐标返回客户区矩形：客户区原点 (0, 0) 的屏幕坐标，加上客户区的宽和高。不含标题栏和边框。

调用顺序固定为：先 `ClientToScreen(0, 0)`，再 `ClientRect`。任一次调用失败时，四个整数均为 0 并返回该错误；第一次调用失败后不会到达第二次调用。最小化窗口沿用现有的 `"window is minimized"` 错误。

坐标使用调用线程当前已有的 DPI 感知。要让 `ClientInfo` 与 `screen.CaptureRegion` 对齐，进程必须与截图使用同一种 DPI 感知；`ClientInfo` 自身不调用 `EnablePerMonitorDPI`。四个返回值来自两次独立的 Win32 调用，不是原子快照。

`EnablePerMonitorDPI` 只应在进程启动早期调用。DPI 感知一旦设定便不能再可靠地改回。

## 探针与任务级输入

`ProbeTarget` 报告一个窗口的事实：进程文件名、该窗口自身的类名、从调用方看的完整性（`target_higher`、`equal`、`target_lower`、`unknown`），以及调用方 UIAccess（`enabled`、`disabled`、`unknown`）。警告只有 `uipi_likely_blocked`、`integrity_unknown`、`uiaccess_unknown`。它不推荐后端，不初始化 HID，也不修改 `SetBackend`。

`ProbeHID` 返回 `initialized`、`dll_unavailable`、`driver_unavailable`、`devices_unavailable` 或 `unknown`。初始化失败仍然返回状态，且不修改全局后端。`SetBackend` 与 `NewInput(BackendHID)` 保持今天的公开错误。

`NewInput` 只接受 `BackendMessage` 或 `BackendHID`。其它整型在初始化 HID 之前返回 `ErrInvalidBackend`。`ForWindow` 与 `Desktop` 把窗口操作和屏幕操作分开。执行器不读取全局后端。包级 `PasteByClipboard` 仍是全局后端兼容入口；`DesktopInput.PasteByClipboard` 是固定后端形式。光标查询、窗口查找、文本读取和 DPI 仍走原有函数。

