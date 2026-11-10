# winput

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/rpdg/winput)

**winput** 是一个轻量级、高性能的 Go 语言 Windows 后台输入自动化库。

它提供了一套统一的、以窗口为中心的 API，抽象了底层的输入机制，支持在标准的 Windows 消息 (`PostMessage`) 和内核级注入 (`Interception` 驱动) 之间无缝切换。

## 定位

`winput` 现在有两层明确定位：
- 面向 Go 开发者的 Windows 输入自动化底层库
- 面向外部 AI 的 MCP adapter 层

对 Go 使用者来说，核心库 API 仍然是主契约。
如果你希望把 `winput` 直接暴露给 AI agent，建议使用仓库内的 MCP adapter，而不是把 agent runtime 语义直接塞进核心库 API。



## 功能特性

*   **纯 Go 实现 (无 CGO)**: 使用动态 DLL 加载，编译不需要 GCC 环境。
*   **窗口为中心 (Window-Centric)**: 所有操作均基于 `Window` 对象，无需直接操作 HWND。
*   **读取控件文本**: `Text()` 可读取标准 Win32 文本控件，`Value()` 会在必要时回退到 UI Automation 以适配更多现代 UI。
*   **后台输入 (Background Input)**:
    *   **消息后端 (Message Backend)**: 直接向窗口消息队列发送事件。无需窗口焦点，也不移动物理鼠标。
    *   **HID 后端 (HID Backend)**: 使用 [Interception](https://github.com/oblitum/Interception) 驱动模拟底层硬件输入。
    *   **键入间隔 (Typing Intervals)**: `Press` 接受可选的按住时间，`Type` 接受可选的字符间隙；`Type(text, 0)` 去掉睡眠，最后一个字符之后不再多睡。
*   **剪贴板粘贴 (Clipboard Paste)**: `PasteByClipboard(text)` 向当前前台窗口发送 Ctrl+V，结束后恢复先前的 Unicode 剪贴板文本。返回 nil 不表示前台已经粘上新文本。它跟随 `SetBackend` 的全局后端，不跟随任务级 `Input`。
*   **环境事实**: `ProbeTarget` 报告一个窗口的可执行文件名、该窗口自身的类名、完整性关系和调用方 UIAccess，以及至多三条警告。它不选择后端，也不初始化 HID。`ProbeHID` 报告本进程能否初始化 HID，且不修改全局后端。`initialized` 不表示某个窗口会收下输入。
*   **任务级输入**: `NewInput` 绑定 `BackendMessage` 或 `BackendHID`。窗口操作用 `WindowInput`，屏幕和无窗口操作用 `DesktopInput`。之后的 `SetBackend` 不会改变这个执行器。窗口查找、文本读取、光标位置和 DPI 仍走原有函数。
*   **坐标管理**:
    *   统一使用 **客户端坐标 (Client Coordinate)** 系统。
    *   内置 `ScreenToClient` / `ClientToScreen` 转换。
    *   `Window.ClientInfo` 以屏幕坐标返回客户区矩形；当调用线程的 DPI 感知一致时，可与 `screen.CaptureRegion` 对齐。
    *   **DPI 感知**: 支持 Per-Monitor DPI 缩放处理。
*   **安全可靠**:
    *   **并发安全 (Thread-Safe)**: 全局输入序列化 (`inputMutex`) 确保操作原子性。
    *   显式错误返回 (Explicit Errors)，拒绝静默失败。
    *   类型安全的 Key 定义 (避免直接使用裸扫描码)。
*   **键盘布局**:
    *   `KeyFromRune` 和 `Type` 函数目前假定为 **美国 QWERTY** 键盘布局。



## 视觉自动化 (Electron / 游戏)

适用于窗口句柄不可靠或需要基于图像识别的操作场景。

```go
import (
	"github.com/rpdg/winput"
	"github.com/rpdg/winput/screen"
)

func main() {
	// 必须开启 DPI 感知以保证坐标精确
	winput.EnablePerMonitorDPI()

	// 1. 捕获整个虚拟桌面（包含所有显示器）。
	// 下面的 ImageToVirtual 只适用于这张全屏图。
	img, err := screen.CaptureVirtualDesktop()
	// 区域图的 (0, 0) 是裁剪矩形的左上角，不是虚拟桌面原点。
	// 不要把区域图交给 ImageToVirtual。
	// img, err = screen.CaptureRegion(0, 0, 1920, 1080)
	// img, err = screen.CaptureRegionWithOptions(0, 0, 1920, 1080, screen.CaptureOptions{})
	if err != nil {
		panic(err)
	}
	// img 是标准的 *image.RGBA，可直接用于 OpenCV/GoCV

	// 2. 执行你的图像匹配逻辑 (伪代码)
	// matchX, matchY := yourCVLib.Match(img, template)

	// 3. 将图片坐标转换为虚拟桌面绝对坐标
	targetX, targetY := screen.ImageToVirtual(int32(matchX), int32(matchY))

	// 4. 移动并点击
	winput.MoveMouseTo(targetX, targetY)
	winput.ClickMouseAt(targetX, targetY)
}
```



## API 参考手册

## MCP Server

本仓库还包含一个最小可用的 MCP server adapter，位于 [cmd/mcp-server](cmd/mcp-server)。
它通过 `stdio JSON-RPC + Content-Length` 帧方式，对外暴露一组稳定的低层 tool。

只读模式启动：

```bash
go run ./cmd/mcp-server
```

开启会修改状态的 tool，例如 `click`、`type_text`：

```bash
go run ./cmd/mcp-server -allow-mutations
```

同时开启修改类和敏感类 tool，例如 `switch_backend`：

```bash
go run ./cmd/mcp-server -allow-mutations -allow-sensitive
```

当前 MCP tool 列表：
- `find_window`
- `find_child`
- `click`
- `double_click`
- `right_click`
- `move_mouse`
- `type_text`
- `press_key`
- `press_hotkey`
- `read_text`
- `capture_screen`
- `switch_backend`
- `get_cursor_pos`
- `get_virtual_bounds`
- `move_mouse_to`
- `click_at`
- `double_click_at`
- `scroll_wheel`
- `scroll`

补充说明：
- discovery 类 tool 返回的是不透明 `target_id`，不会直接暴露 HWND
- `find_window` 和 `find_child` 会尽量返回稳定的运行时元数据，例如可见性、client 尺寸、DPI
- `find_window` 接受 `pid`；按进程名或 PID 查找时用 `index` 选择候选并回传 `count`
- 屏幕坐标类 tool（`move_mouse_to`、`click_at`、`double_click_at`、`scroll_wheel`）使用虚拟桌面绝对坐标；`scroll` 是绑定窗口的定向滚轮
- server 启动时会开启每显示器 DPI 感知，默认进程下 `capture_screen` 可用
- tool 元数据里包含 `inputSchema`、`outputSchema`、`errorSchema`
- 默认会拦截修改类和敏感类 tool
- 支持 `ping` 与协议版本协商

相关文档：
- adapter 设计与决策：[docs/mcp-tools.md](docs/mcp-tools.md)
- 协议会话示例：[docs/mcp-example-session.md](docs/mcp-example-session.md)
- server 入口说明：[cmd/mcp-server/README.md](cmd/mcp-server/README.md)



## 后端限制与权限

### 消息后端 (Message Backend)
*   **机制**: 使用 `PostMessageW`。
*   **优点**: 无需焦点，不占用鼠标，真正的后台运行。
*   **缺点**:
    *   **组合键限制**: `PostMessage` **不会** 更新全局键盘状态。依赖 `GetKeyState` 的程序（如检测 Ctrl+C）可能无法识别修饰键。
        `Window.PressHotkey` 现在会在这类组合键场景下临时回退为前台键盘事件，
        因此这类调用不再是纯后台操作。
    *   **UIPI 限制**: 无法向权限高于自己的窗口（如管理员运行的程序）发送消息，除非自己也以管理员运行。
    *   **坐标限制**: 受限于 16 位有符号整数 ([-32768, 32767])。超出范围的坐标会被截断。
    *   **兼容性**: 部分游戏 (DirectX/OpenGL/RawInput) 和 UI 框架可能忽略消息。

### HID 后端 (HID Backend)
*   **机制**: 使用 Interception 驱动（内核级）。
*   **上下文**: 使用全局唯一的驱动上下文（单例）。适合自动化脚本，集成到大型应用时需注意。
*   **优点**: 兼容性极强，支持游戏和反作弊保护，模拟真实的硬件信号。
*   **缺点**:
    *   **依赖驱动**: 必须安装 Interception 驱动。
    *   **阻塞调用**: `Move` 操作是同步且阻塞的（为了模拟人类速度）。
    *   **鼠标移动**: 会实际移动物理光标。
    *   **焦点**: 窗口上的按下、`Press`、`PressHotkey` 和 `Type` 只在该窗口的顶层祖先已是前台时发送。否则返回 `ErrTargetNotForeground`，并且不按下这一键。`KeyUp` 仍会发送，以便松开已经按下的键。全局键盘方法不做这次检查。



## 安装

```bash
go get github.com/rpdg/winput
```

### HID 支持 (可选)
本库为 **纯 Go 实现**，**不需要** CGO 编译环境。
若需使用 HID 后端：
1.  安装 **Interception 驱动**。
2.  确保 `interception.dll` 存在。默认在当前目录或 PATH 中查找，也可以指定路径：
    ```go
    // 必须在第一次成功的 ProbeHID、NewInput(BackendHID) 或 SetBackend(BackendHID) 之前调用。
    // 那一次成功会把 DLL 路径冻结到进程结束。
    winput.SetHIDLibraryPath("libs/interception.dll")
    if err := winput.SetBackend(winput.BackendHID); err != nil {
        // 处理错误（例如回退到消息后端）
        panic(err)
    }
    ```



## 使用示例

### 1. 基础后台交互 (Message Backend)
适用于标准窗口（如记事本）。后台运行，不占用鼠标。
**提示**: 对于某些应用（如记事本），你可能需要查找子窗口（如 "Edit" 类）才能发送文本。
```bash
go run cmd/example/basic_message/main.go
```

### 2. 全局视觉自动化 (Electron / 游戏)
适用于 HWND 不可靠的应用 (VS Code, Discord)。使用绝对屏幕坐标。
**提示**: 使用 `screen.ImageToVirtual(x, y)` 将 OpenCV 截图坐标转换为 winput 坐标。
```bash
go run cmd/example/global_vision/main.go
```

### 3. HID 硬件模拟
模拟物理硬件输入。需要驱动。
```bash
go run cmd/example/advanced_hid/main.go
```



## 快速开始 (代码片段)

```go
package main

import (
	"log"
	"github.com/rpdg/winput"
)

func main() {
	// 1. 查找目标窗口
	w, err := winput.FindByTitle("无标题 - 记事本")
	if err != nil {
		log.Fatal(err)
	}

	edit, err := w.FindChildByClass("Edit")
	if err != nil {
		log.Fatal(err)
	}

	// 2. 点击 (左键)
	if err := edit.Click(100, 100); err != nil {
		log.Fatal(err)
	}

	// 3. 输入文本
	edit.Type("Hello World")
	edit.Press(winput.KeyEnter)

	// 4. 读取标准文本控件内容
	value, err := edit.Text()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("当前文本:", value)

	// 5. 全局输入 (不针对特定窗口)
	winput.Type("Hello Electron!")
	winput.Press(winput.KeyEnter)

	// 6. 使用 winput/screen (查询屏幕边界)
	// import "github.com/rpdg/winput/screen"
	bounds := screen.VirtualBounds()
	fmt.Printf("桌面边界: %d, %d\n", bounds.Right, bounds.Bottom)
}
```



## 错误处理指南

`winput` 拒绝静默失败。以下是您应该处理的常见错误：

| 错误变量 | 描述 | 处理建议 |
| :--- | :--- | :--- |
| `ErrWindowNotFound` | 无法通过 Title/Class/PID 找到窗口。 | 检查应用是否运行，或尝试改用 `FindByClass`。 |
| `ErrDriverNotInstalled` | Interception 驱动上下文打不开（仅 HID）。 | 提示用户安装驱动，或继续使用 Message 后端。 |
| `ErrDLLLoadFailed` | 库找不到、库里缺少符号，或鼠标和键盘设备都没有。 | 检查 `SetHIDLibraryPath`、DLL 和驱动设备。这不是 `ErrDriverNotInstalled`。 |
| `ErrTargetChanged` | `ProbeTarget` 期间窗口还在，但进程、线程，或开始时已经读到的类名变了。 | 重新探针。不会返回半份结果。 |
| `ErrInvalidBackend` | `NewInput` 收到了 `BackendMessage` 和 `BackendHID` 以外的值。 | 只传这两个值。`BackendMessage` 是 `0`，它是合法的。 |
| `ErrInputInvalid` | 执行器或操作面不是 `NewInput`、`ForWindow` 或 `Desktop` 得到的。 | 使用它们的返回值。`ForWindow(nil)` 要到调用时才返回 `ErrWindowGone`。 |
| `ErrUnsupportedKey` | 字符无法映射到按键。 | 检查输入字符串，特殊按键请使用 `KeyDown`。 |
| `ErrPermissionDenied` | 操作被系统阻止 (如 UIPI)。 | 尝试以管理员身份运行程序。 |

健壮的错误处理示例：

```go
// SetBackend 现在会立即检查驱动环境。
if err := winput.SetBackend(winput.BackendHID); err != nil {
    log.Printf("HID 后端不可用: %v。将继续使用默认的消息后端。", err)
    // 默认即为 BackendMessage，无需额外设置。
}

// 之后的调用将使用成功设置的后端。
if err := w.Click(100, 100); err != nil {
    log.Fatal(err)
}
```



## 高级用法

### 1. 处理高 DPI 显示器
现代 Windows 会对应用进行缩放。为了确保您的 `(100, 100)` 点击准确落在目标像素上：

```go
// 在程序启动时调用
if err := winput.EnablePerMonitorDPI(); err != nil {
    log.Printf("DPI 设置失败: %v", err)
}

// 检查特定窗口的 DPI (96 为标准 100%)
dpi, _ := w.DPI()
fmt.Printf("目标窗口 DPI: %d (缩放比: %.2f%%)\n", dpi, float64(dpi)/96.0*100)
```

### 2. HID Backend with Fallback
在游戏或反作弊场景使用 HID，在普通应用使用 Message。

```go
// 尝试启用 HID 后端
if err := winput.SetBackend(winput.BackendHID); err != nil {
    log.Println("HID 初始化失败，使用默认的消息后端:", err)
    // 无需额外操作，默认即为 BackendMessage
}

// 消息后端把文本送进 w。HID 在 w 的顶层窗口还不是前台时返回 ErrTargetNotForeground。
w.Type("password")
```

### 3. 探针和任务级执行器
`ProbeTarget` 和 `ProbeHID` 只报告事实，不选择后端。`ProbeHID` 可以加载驱动；失败仍然是一个状态，全局后端保持原样。

```go
report, err := winput.ProbeTarget(w)
if err != nil {
    log.Fatal(err)
}
status := winput.ProbeHID()
log.Printf("class=%s hid=%s", report.Features.TargetClass, status)

input, err := winput.NewInput(winput.BackendMessage)
if err != nil {
    log.Fatal(err)
}
win, err := input.ForWindow(w)
if err != nil {
    log.Fatal(err)
}
if err := win.Type("123456"); err != nil {
    log.Fatal(err)
}
if _, err := input.Desktop(); err != nil {
    log.Fatal(err)
}
```

`status` 是 `initialized`、`dll_unavailable`、`driver_unavailable`、`devices_unavailable` 或 `unknown` 之一。`PasteByClipboard` 仍使用 `SetBackend`。

### 4. 按键映射细节
`winput` 将 rune 映射为扫描码 (Scan Code Set 1)。
- **支持范围**: A-Z, 0-9, 常用符号 (`!`, `@`, `#`...), 空格, 回车, Tab。
- **自动 Shift**: `Type("A")` 会自动发送 `Shift 按下` -> `a 按下` -> `a 抬起` -> `Shift 抬起`。



## 项目对比

| 特性         | winput (Go)                | C# Interceptor 封装   | Python winput (ctypes) |
| :----------- | :------------------------- | :-------------------- | :--------------------- |
| **后端支持** | **双引擎 (HID + Message)** | 仅 HID (Interception) | 仅 Message (User32)    |
| **API 风格** | 面向对象 (`w.Click`)       | 底层 (`SendInput`)    | 函数式                 |
| **依赖项**   | 无 (默认) / 驱动 (HID)     | 必须安装驱动          | 无                     |
| **安全性**   | 显式错误返回               | 异常 / 静默失败       | 静默 / 返回码          |
| **DPI 感知** | ✅ 支持                     | ❌ 需手动计算          | ❌ 需手动计算           |

*   **对比 Python winput**: Python 版适合简单自动化，但缺乏游戏或顽固应用所需的内核级注入能力。

*   **对比 C# Interceptor**: 大多数 C# 封装直接暴露原始驱动 API，而 `winput` 将其抽象为高级动作 (Click, Type) 并内置了坐标转换逻辑。

	

## 选型指南:  winput vs robotgo

| 核心能力 | robotgo | winput |
| :--- | :--- | :--- |
| **输入层级** | 操作系统合成层 (`SendInput`) | **驱动层** (`Interception`) + 消息层 |
| **后台操作** | ❌ 不支持 | ✅ **原生支持** (PostMessage 机制) |
| **游戏/反作弊兼容** | ❌ 易被拦截 | ✅ **驱动级** |
| **多屏负坐标语义** | 隐式/依赖设备 | **显式虚拟桌面坐标** (原生支持多屏/负坐标/DPI) |
| **OpenCV/RPA 对齐** | 基础 | **原生设计** (截图->映射->输入 闭环) |
| **跨平台** | ✅ (Win/Mac/Linux) | ❌ (Windows 专用) |
| **定位** | 快速脚本, 简单 Demo | **工程级自动化**, 游戏, Electron, 后台任务 |

**一句话总结**:

*   **robotgo** 是 **"自动化工具"**：适合跨平台、快速上手的场景。

*   **winput** 是 **"输入基础设施"**：专为解决 **Electron**、**游戏**、**后台窗口** 及 **拒绝 SendInput** 等"硬骨头"场景而生，追求输入的**确定性**与**可靠性**。



## 许可证

MIT
