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
*   [func KeyDown](#func-keydown)
*   [func KeyUp](#func-keyup)
*   [func Press](#func-press)
*   [func PressHotkey](#func-presshotkey)
*   [func Type](#func-type)
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
func SetBackend(b Backend)
```
SetBackend 配置全局输入注入方法。

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
Press 模拟一次全局按键（按下+抬起）。

### func PressHotkey

```go
func PressHotkey(keys ...Key) error
```
PressHotkey 模拟全局组合键。

### func Type

```go
func Type(text string) error
```
Type 模拟全局文本输入（通过模拟按键序列）。

## 类型

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
Scroll 在指定坐标执行鼠标滚轮滚动。

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

#### func (*Window) Type

```go
func (w *Window) Type(text string) error
```
输入字符串，自动处理大写字母和符号的 Shift 切换。

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
