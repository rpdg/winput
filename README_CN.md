# winput

**winput** 是一个轻量级、高性能的 Go 语言 Windows 后台输入自动化库。

它提供了一套统一的、以窗口为中心的 API，抽象了底层的输入机制，支持在标准的 Windows 消息 (`PostMessage`) 和内核级注入 (`Interception` 驱动) 之间无缝切换。

## 功能特性

*   **纯 Go 实现 (无 CGO)**: 使用动态 DLL 加载，编译不需要 GCC 环境。
*   **窗口为中心 (Window-Centric)**: 所有操作均基于 `Window` 对象，无需直接操作 HWND。
*   **后台输入 (Background Input)**:
    *   **消息后端 (Message Backend)**: 直接向窗口消息队列发送事件。无需窗口焦点，也不移动物理鼠标。
    *   **HID 后端 (HID Backend)**: 使用 [Interception](https://github.com/oblitum/Interception) 驱动模拟底层硬件输入。
*   **坐标管理**:
    *   统一使用 **客户端坐标 (Client Coordinate)** 系统。
    *   内置 `ScreenToClient` / `ClientToScreen` 转换。
    *   **DPI 感知**: 支持 Per-Monitor DPI 缩放处理。
*   **安全可靠**:
    *   显式错误返回 (Explicit Errors)，拒绝静默失败。
    *   类型安全的 Key 定义 (避免直接使用裸扫描码)。

## 后端限制与权限

### 消息后端 (Message Backend)
*   **机制**: 使用 `PostMessageW`。
*   **优点**: 无需焦点，不占用鼠标，真正的后台运行。
*   **缺点**:
    *   **组合键限制**: `PostMessage` **不会** 更新全局键盘状态。依赖 `GetKeyState` 的程序（如检测 Ctrl+C）可能无法识别修饰键。
    *   **UIPI 限制**: 无法向权限高于自己的窗口（如管理员运行的程序）发送消息，除非自己也以管理员运行。
    *   **兼容性**: 部分游戏 (DirectX/OpenGL) 和 UI 框架可能忽略消息。

### HID 后端 (HID Backend)
*   **机制**: 使用 Interception 驱动（内核级）。
*   **上下文**: 使用全局唯一的驱动上下文（单例）。适合自动化脚本，集成到大型应用时需注意。
*   **优点**: 兼容性极强，支持游戏和反作弊保护，模拟真实的硬件信号。
*   **缺点**:
    *   **依赖驱动**: 必须安装 Interception 驱动。
    *   **阻塞调用**: `Move` 操作是同步且阻塞的（为了模拟人类速度）。
    *   **鼠标移动**: 会实际移动物理光标。
    *   **焦点**: 通常需要窗口处于前台才能正确响应输入。

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
    winput.SetHIDLibraryPath("libs/interception.dll")
    winput.SetBackend(winput.BackendHID)
    ```

## 快速开始

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

	// 2. 点击 (左键)
	if err := w.Click(100, 100); err != nil {
		log.Fatal(err)
	}

	// 3. 输入文本
	w.Type("Hello World")
	w.Press(winput.KeyEnter)
}
```

## 错误处理指南

`winput` 拒绝静默失败。以下是您应该处理的常见错误：

| 错误变量 | 描述 | 处理建议 |
| :--- | :--- | :--- |
| `ErrWindowNotFound` | 无法通过 Title/Class/PID 找到窗口。 | 检查应用是否运行，或尝试改用 `FindByClass`。 |
| `ErrDriverNotInstalled` | Interception 驱动丢失（仅 HID 模式）。 | 提示用户安装驱动，或自动降级到 Message 后端。 |
| `ErrDLLLoadFailed` | `interception.dll` 加载失败。 | 检查 DLL 路径 (`SetHIDLibraryPath`) 或安装。 |
| `ErrUnsupportedKey` | 字符无法映射到按键。 | 检查输入字符串，特殊按键请使用 `KeyDown`。 |
| `ErrPermissionDenied` | 操作被系统阻止 (如 UIPI)。 | 尝试以管理员身份运行程序。 |

健壮的错误处理示例：

```go
// 尝试切换到 HID 模式
winput.SetBackend(winput.BackendHID)

// 执行动作
err := w.Click(100, 100)

// 检查是否是因为驱动未安装
if errors.Is(err, winput.ErrDriverNotInstalled) {
    log.Println("HID 驱动未安装，降级到消息后端...")
    winput.SetBackend(winput.BackendMessage)
    w.Click(100, 100) // 重试
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

### 2. HID 后端与自动降级
在游戏或反作弊场景使用 HID，在普通应用使用 Message。

```go
winput.SetBackend(winput.BackendHID)
err := w.Type("password")
if err != nil {
    // 如果 HID 失败，切回 Message 模式
    winput.SetBackend(winput.BackendMessage)
    w.Type("password")
}
```

### 3. 按键映射细节
`winput` 将 rune 映射为扫描码 (Scan Code Set 1)。
- **支持范围**: A-Z, 0-9, 常用符号 (`!`, `@`, `#`...), 空格, 回车, Tab。
- **自动 Shift**: `Type("A")` 会自动发送 `Shift 按下` -> `a 按下` -> `a 抬起` -> `Shift 抬起`。

## 项目对比

| 特性 | winput (Go) | C# Interceptor 封装 | Python winput (ctypes) |
| :--- | :--- | :--- | :--- |
| **后端支持** | **双引擎 (HID + Message)** | 仅 HID (Interception) | 仅 Message (User32) |
| **API 风格** | 面向对象 (`w.Click`) | 底层 (`SendInput`) | 函数式 |
| **依赖项** | 无 (默认) / 驱动 (HID) | 必须安装驱动 | 无 |
| **安全性** | 显式错误返回 | 异常 / 静默失败 | 静默 / 返回码 |
| **DPI 感知** | ✅ 支持 | ❌ 需手动计算 | ❌ 需手动计算 |

*   **对比 Python winput**: Python 版适合简单自动化，但缺乏游戏或顽固应用所需的内核级注入能力。
*   **对比 C# Interceptor**: 大多数 C# 封装直接暴露原始驱动 API，而 `winput` 将其抽象为高级动作 (Click, Type) 并内置了坐标转换逻辑。

## 许可证

MIT