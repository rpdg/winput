# winput

**winput** 是一个轻量级、高性能的 Go 语言 Windows 后台输入自动化库。

它提供了一套统一的、以窗口为中心的 API，抽象了底层的输入机制，支持在标准的 Windows 消息 (`PostMessage`) 和内核级注入 (`Interception` 驱动) 之间无缝切换。

## 功能特性

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

## 安装

```bash
go get github.com/rpdg/winput
```

### HID 后端的前置要求
如果您打算使用 `BackendHID` (Interception 模式)，您必须：
1.  安装 **Interception 驱动** (运行官方发布包中的 `install-interception.exe`)。
2.  确保 `interception.dll` 位于您的应用程序工作目录或系统 PATH 中。
3.  开启 CGO (需要 MinGW 等 C 编译器)。

> **注意**: 默认的 `BackendMessage` **不需要** 驱动或 CGO (运行时不需要，尽管本库在编译时链接了 HID 后端)。

## 使用方法

### 基础示例 (后台消息模式)

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
		log.Fatal("未找到窗口:", err)
	}

	// 2. 点击窗口内的 (100, 100) 位置
	// 这不会移动物理鼠标光标。
	if err := w.Click(100, 100); err != nil {
		log.Fatal(err)
	}

	// 3. 输入文本
	w.Type("Hello Background World")
	w.Press(winput.KeyEnter)
}
```

### 切换到 HID 后端

如果目标应用程序屏蔽了 `PostMessage`，或者您需要模拟真实的硬件行为（例如游戏或反作弊程序），请使用此模式。

```go
func main() {
    w, _ := winput.FindByClass("Notepad")

    // 将全局后端切换为 HID
    // 注意: 这需要安装 Interception 驱动。
    // 初始化错误将在第一次执行动作时返回。
    winput.SetBackend(winput.BackendHID)

    // 现在这将会物理移动鼠标光标到窗口内的 (100, 100)
    err := w.Click(100, 100)
    if err != nil {
        log.Fatalf("HID 输入失败 (驱动未安装?): %v", err)
    }
}
```

## 架构

### 包结构
```
winput/
├── window/      # Win32 窗口 & DPI API
├── mouse/       # PostMessage 鼠标实现
├── keyboard/    # PostMessage 键盘实现
├── hid/         # Interception 驱动包装 & 逻辑
│   └── interception/ # 底层 CGO 绑定
└── winput.go    # 公共 API & 后端切换
```

### 设计原则
1.  **坐标一致性**: API *始终* 接受窗口客户端 (Client) 坐标。后端负责在必要时将其转换为屏幕坐标（例如用于 HID 注入）。
2.  **显式失败**: 如果操作无法执行（例如窗口已关闭、后端不可用），将返回特定的错误。
3.  **零成本抽象**: 默认的消息后端是轻量级的纯 Go 实现 (syscall)。只有在请求时才会加载 HID 后端逻辑。

## 许可证

MIT
