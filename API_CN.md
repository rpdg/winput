# winput API 参考手册

`winput` 包提供了一个用于 Windows 后台输入自动化的高级接口。

## 索引

*   [变量](#变量)
*   [常量](#常量)
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

## 变量

```go
var (
    // ErrWindowNotFound 意味着无法通过标题、类名或 PID 找到目标窗口。
    ErrWindowNotFound     = errors.New("window not found")     // 未找到窗口
    ErrWindowGone         = errors.New("window is gone")       // 窗口句柄失效
    ErrWindowNotVisible   = errors.New("window is not visible")// 窗口不可见或最小化
    ErrUnsupportedKey     = errors.New("unsupported key")      // 不支持的按键

    // ErrBackendUnavailable 意味着所选的后端（如 HID）初始化失败。
    ErrBackendUnavailable = errors.New("input backend unavailable")

    // ErrDriverNotInstalled 是 BackendHID 特有的，意味着 Interception 驱动丢失或不可访问。
    ErrDriverNotInstalled = errors.New("interception driver not installed or accessible")

    // ErrDLLLoadFailed 意味着 interception.dll 加载失败。
    ErrDLLLoadFailed = errors.New("failed to load interception library")

    // ErrPermissionDenied 意味着操作因系统权限限制（如 UIPI）而失败。
    ErrPermissionDenied = errors.New("permission denied")
)
```

## 常量

### 后端常量 (Backend Constants)

```go
const (
    // BackendMessage 使用标准的 Windows 消息 (PostMessage) 进行输入。
    // 它在后台工作，不需要窗口焦点。
    BackendMessage Backend = iota

    // BackendHID 使用 Interception 驱动程序模拟硬件输入。
    // 需要系统上安装 Interception 驱动。
    // 通过此后端进行的输入将移动物理光标，且无法与真实硬件输入区分。
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
    // ... 以及更多标准按键
)
```

## 函数

### func EnablePerMonitorDPI

```go
func EnablePerMonitorDPI() error
```
EnablePerMonitorDPI 将当前进程设置为 Per-Monitor (v2) DPI 感知。这确保了在高 DPI 设置下坐标计算 (ScreenToClient/ClientToScreen) 的准确性。建议在程序启动时调用此函数。

### func SetBackend

```go
func SetBackend(b Backend)
```
SetBackend 配置全局输入注入方法。默认为 `BackendMessage`。
如果选择了 `BackendHID`，初始化检查（驱动是否存在）将推迟到首次尝试输入动作时进行。

### func SetHIDLibraryPath

```go
func SetHIDLibraryPath(path string)
```
SetHIDLibraryPath 设置 `interception.dll` 的自定义加载路径。
默认情况下，库会在系统 PATH 或当前目录下查找 DLL。
必须在启用 `BackendHID` **之前** 调用此函数。

## 类型

### type Backend

```go
type Backend int
```
Backend 代表用于输入注入的底层机制。

### type Key

```go
type Key = uint16
```
Key 代表硬件扫描码 (Scan Code)。它避免使用虚拟键码 (VK)，以确保与底层钩子和游戏更好的兼容性。

#### func KeyFromRune

```go
func KeyFromRune(r rune) (Key, bool)
```
KeyFromRune 尝试将 unicode 字符 (rune) 映射到相应的扫描码 `Key`。它支持基本的 ASCII 字符。如果无法映射该字符，则返回 false。

### type Window

```go
type Window struct {
    HWND uintptr
}
```
Window 代表自动化的目标窗口。它封装了窗口句柄 (HWND)，并提供了输入和坐标管理的方法。

#### func FindByPID

```go
func FindByPID(pid uint32) ([]*Window, error)
```
FindByPID 返回属于指定进程 ID 的所有顶级窗口。

#### func FindByProcessName

```go
func FindByProcessName(name string) ([]*Window, error)
```
FindByProcessName 返回属于指定可执行文件名称（例如 "notepad.exe"）的所有顶级窗口。

#### func FindByTitle

#### func (*Window) Move

```go
func (w *Window) Move(x, y int32) error
```
Move 将鼠标光标移动到**相对于窗口客户区**的指定坐标。
- **消息后端**: 投递 `WM_MOUSEMOVE` 消息（瞬间完成）。
- **HID 后端**: 计算绝对屏幕位置并物理移动鼠标光标（拟人化轨迹）。**此操作是同步且阻塞的。**

#### func (*Window) Click

```go
func (w *Window) Click(x, y int32) error
```
Click 在指定的客户区坐标执行鼠标左键点击。它会自动先将光标移动到目标位置。

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
Scroll 在指定坐标执行鼠标滚轮滚动。
`delta` 表示滚动量；120 代表标准滚轮的一格。正值表示向前/向上滚动，负值表示向后/向下滚动。

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
Press 模拟一次完整的按键过程 (KeyDown 后跟 KeyUp)。
在 `BackendHID` 模式下，按下和抬起之间会插入随机延迟以模拟人类速度。

#### func (*Window) Type

```go
func (w *Window) Type(text string) error
```
输入字符串，自动处理大写字母和符号的 Shift 切换。


#### func (*Window) DPI

```go
func (w *Window) DPI() (uint32, error)
```
DPI 返回窗口的每英寸点数 (DPI) 设置。标准 DPI 为 96。
它会尝试使用 Per-Monitor V2 API，在旧系统上会降级使用系统 DPI 或 GDI DeviceCaps。

#### func (*Window) ClientRect

```go
func (w *Window) ClientRect() (width, height int32, err error)
```
ClientRect 返回窗口客户区（不包括边框和标题栏）的宽度和高度。

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