# winput


---
<p align="center">
 <a href="README_CN.md">🇨🇳 中文</a> 
</p>
---

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/rpdg/winput)


**winput** is a lightweight, high-performance Go library for Windows background input automation.

It provides a unified, window-centric API that abstracts the underlying input mechanism, allowing seamless switching between standard Window Messages (`PostMessage`) and kernel-level injection (`Interception` driver).

## Positioning

`winput` has two intended layers:
- a focused Go library for Windows input automation
- an in-repo MCP adapter layer for exposing stable low-level tools to external AI systems

The core library remains the primary contract for Go developers.
If you want to expose `winput` directly to AI agents, use the MCP adapter instead of embedding agent runtime semantics into the library API.

## Features

*   **Pure Go (No CGO)**: Uses dynamic DLL loading. No GCC required for compilation.
*   **Window-Centric API**: Operations are performed on `Window` objects, not raw HWNDs.
*   **Read Text from Controls**: Use `Text()` for standard Win32 controls and `Value()` for best-effort reads with UI Automation fallback on modern controls.
*   **Background Input**: 
    *   **Message Backend**: Sends inputs directly to window message queues. Works without window focus or mouse cursor movement.
    *   **HID Backend**: Uses the [Interception](https://github.com/oblitum/Interception) driver to simulate hardware input at the kernel level.
    *   **Typing Intervals**: `Press` takes an optional hold time and `Type` an optional per-character gap; `Type(text, 0)` removes the sleep and the last character is not followed by a trailing sleep.
*   **Clipboard Paste**: `PasteByClipboard(text)` sends Ctrl+V to the current foreground window and restores the previous Unicode clipboard text afterwards. A nil return does not mean the foreground accepted the text. It follows the global backend from `SetBackend`, not a task-level `Input`.
*   **Environment facts**: `ProbeTarget` reports one window's executable file name, that window's own class name, the integrity relation, and caller UIAccess, plus up to three warnings. It does not choose a backend or initialize HID. `ProbeHID` reports whether this process can initialize HID and does not change the global backend. `initialized` does not mean a window will accept the input.
*   **Task-level input**: `NewInput` binds `BackendMessage` or `BackendHID`. Window calls use `WindowInput`; screen and windowless calls use `DesktopInput`. A later `SetBackend` does not change that executor. Window search, text reads, cursor position, and DPI stay on the existing functions.
*   **Coordinate Management**:
    *   Unified **Client Coordinate** system for all APIs.
    *   Built-in `ScreenToClient` / `ClientToScreen` conversion.
    *   `Window.ClientInfo` returns the client-area rectangle in screen coordinates, aligned with `screen.CaptureRegion` when the calling thread's DPI awareness matches.
    *   **DPI Awareness**: Helpers for Per-Monitor DPI scaling.
*   **Safety & Reliability**:
    *   **Thread-Safe**: Global input serialization ensures atomic operations (`inputMutex`).
    *   Explicit error returns (no silent failures).
    *   Type-safe Key definitions.
*   **Keyboard Layout**:
    *   The `KeyFromRune` and `Type` functions currently assume a **US QWERTY** keyboard layout.

## Vision Automation (Electron / Games)

Ideal for applications where window handles are unreliable.

```go
import (
	"github.com/rpdg/winput"
	"github.com/rpdg/winput/screen"
)

func main() {
	winput.EnablePerMonitorDPI()

	// 1. Capture the entire virtual desktop (all monitors).
	// ImageToVirtual below applies only to this full-desktop image.
	img, err := screen.CaptureVirtualDesktop()
	// A region image's (0, 0) is the clipped rectangle, not the virtual
	// desktop origin. Do not pass that image to ImageToVirtual.
	// img, err = screen.CaptureRegion(0, 0, 1920, 1080)
	// img, err = screen.CaptureRegionWithOptions(0, 0, 1920, 1080, screen.CaptureOptions{})
	if err != nil {
		panic(err)
	}
	// img is a standard *image.RGBA, ready for OpenCV/GoCV

	// 2. Perform your CV matching here (pseudo-code)
	// matchX, matchY := yourCVLib.Match(img, template)

	// 3. Convert image coordinates to virtual desktop coordinates
	targetX, targetY := screen.ImageToVirtual(int32(matchX), int32(matchY))

	// 4. Click target (ClickMouseAt automatically moves to coordinates and clicks)
	if err := winput.ClickMouseAt(targetX, targetY); err != nil {
		log.Printf("Click failed: %v", err)
	}
}
```

## API Reference

Comprehensive package documentation and API specifications:
- [API Reference (English)](API.md)
- [API 参考手册 (中文)](API_CN.md)

## MCP Server

This repository also includes a minimal MCP server adapter under [cmd/mcp-server](cmd/mcp-server).
It exposes a stable subset of `winput` capabilities over stdio JSON-RPC with `Content-Length` framing.

Start the server in read-only mode:

```bash
go run ./cmd/mcp-server
```

Enable mutating tools such as `click` and `type_text`:

```bash
go run ./cmd/mcp-server -allow-mutations
```

Enable mutating and sensitive tools such as `switch_backend`:

```bash
go run ./cmd/mcp-server -allow-mutations -allow-sensitive
```

Current MCP tool catalog:
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

Notes:
- discovery tools return an opaque `target_id` instead of raw HWND values
- `find_window` and `find_child` also return stable runtime metadata such as visibility, client size, and DPI when available
- `find_window` accepts `pid` and, for process-name or PID searches, selects a candidate with `index` and reports `count`
- the screen-coordinate tools (`move_mouse_to`, `click_at`, `double_click_at`, `scroll_wheel`) use absolute virtual-desktop coordinates; `scroll` is the window-directed wheel tool
- the server enables per-monitor DPI awareness at startup so `capture_screen` works in the default process
- tool metadata includes `inputSchema`, `outputSchema`, and `errorSchema`
- mutating and sensitive tools are blocked by default
- `ping` and protocol version negotiation are supported

Further MCP details:
- adapter design and decisions: [docs/mcp-tools.md](docs/mcp-tools.md)
- example framed session: [docs/mcp-example-session.md](docs/mcp-example-session.md)
- server entrypoint notes: [cmd/mcp-server/README.md](cmd/mcp-server/README.md)

## Backend Limitations & Permissions

### Message Backend
*   **Mechanism**: Uses `PostMessageW`.
*   **Pros**: No focus required, no mouse movement, works in background.
*   **Cons**: 
    *   **Modifier Keys**: `PostMessage` does **not** update global keyboard state. Apps checking `GetKeyState` (e.g. for Ctrl+C) might fail.
        `Window.PressHotkey` works around this for modifier combinations by temporarily
        using foreground keyboard events, so those calls are not purely background operations.
    *   **UIPI**: Cannot send messages to apps running as Administrator if your app is not.
    *   **Coordinates**: Limited to 16-bit signed integer range ([-32768, 32767]). Larger coordinates will be clipped.
    *   **Compatibility**: Some games (DirectX/OpenGL/RawInput) and frameworks (Qt/WPF) ignore these messages.

### HID Backend
*   **Mechanism**: Uses Interception driver (kernel-level).
*   **Context**: Uses a global driver context (singleton). Safe for automation scripts, but be aware if integrating into larger apps.
*   **Pros**: Works with almost everything (games, anti-cheat), undetectable as software input.
*   **Cons**:
    *   **Driver Required**: Must install Interception driver.
    *   **Blocking**: `Move` operations are synchronous and blocking (to simulate human speed).
    *   **Mouse Movement**: Physically moves the cursor.
    *   **Focus**: Window key-down, `Press`, `PressHotkey`, and `Type` send only while the window's top-level ancestor is the foreground window. Otherwise they return `ErrTargetNotForeground` and do not press that key. `KeyUp` still sends, so a key that is already down can be released. Global keyboard methods are not checked.

## Installation

```bash
go get github.com/rpdg/winput
```

### HID Support (Optional)
This library is **Pure Go** and does **not** require CGO.
To use the HID backend:
1.  Install the **Interception driver**.
2.  Place `interception.dll` in your app directory, or specify its path:
    ```go
    // Call this before the first successful ProbeHID, NewInput(BackendHID), or SetBackend(BackendHID).
    // That success freezes the DLL path for the rest of the process.
    winput.SetHIDLibraryPath("libs/interception.dll")
    if err := winput.SetBackend(winput.BackendHID); err != nil {
        // Handle error (e.g. fallback to Message backend)
        panic(err)
    }
    ```

## Usage Examples

### 1. Basic Message Backend (Standard Apps)
Ideal for standard windows (Notepad, etc.). Works in background.
**Tip**: For some apps (like Notepad), you may need to find the child window (e.g., "Edit") to send text.
```bash
go run cmd/example/basic_message/main.go
```

### 2. Global Vision Automation (Electron / Games)
For apps where HWND is unreliable (VS Code, Discord, Games). Uses absolute screen coordinates.
**Tip**: Use `screen.ImageToVirtual(x, y)` to convert OpenCV screenshot coordinates to winput coordinates.
```bash
go run cmd/example/global_vision/main.go
```

### 3. HID Backend (Hardware Simulation)
Simulates physical hardware input. Requires driver.
```bash
go run cmd/example/advanced_hid/main.go
```

### 4. Task-Level Executor & Probes (Isolated State)
Demonstrates `NewInput`, `ForWindow`, `Desktop`, `ProbeTarget`, and `ProbeHID`.
```bash
go run cmd/example/task_executor/main.go
```

## Quick Start (Code Snippet)

```go
package main

import (
	"fmt"
	"log"

	"github.com/rpdg/winput"
	"github.com/rpdg/winput/screen"
)

func main() {
	// 1. Find target window
	w, err := winput.FindByTitle("Untitled - Notepad")
	if err != nil {
		log.Fatal(err)
	}

	// For standard Win32 Notepad, text is inside the "Edit" child control.
	// Modern Windows 11 Notepad uses WinUI 3; fallback to the main window if not found.
	edit, err := w.FindChildByClass("Edit")
	if err != nil {
		edit = w
	}

	// 2. Click (Left Button)
	if err := edit.Click(100, 100); err != nil {
		log.Fatal(err)
	}

	// 3. Type text
	if err := edit.Type("Hello World"); err != nil {
		log.Fatal(err)
	}
	if err := edit.Press(winput.KeyEnter); err != nil {
		log.Fatal(err)
	}

	// 4. Read text from control (Value provides UI Automation fallback for modern apps)
	value, err := edit.Value()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Current text:", value)

	// 5. Global Input (Target independent)
	if err := winput.Type("Hello Electron!"); err != nil {
		log.Fatal(err)
	}
	if err := winput.Press(winput.KeyEnter); err != nil {
		log.Fatal(err)
	}

	// 6. Using winput/screen (Boundary query)
	bounds := screen.VirtualBounds()
	fmt.Printf("Virtual Desktop Bounds: %d, %d\n", bounds.Right, bounds.Bottom)
}
```

## Error Handling

winput avoids silent failures. Common errors you should handle:

| Error Variable | Description | Handling |
| :--- | :--- | :--- |
| `ErrWindowNotFound` | Window not found by Title/Class/PID. | Check if the app is running or use `FindByClass` as fallback. |
| `ErrWindowGone` | Window handle is closed or no longer valid. | Search for the window again. |
| `ErrWindowNotVisible` | Window is minimized or hidden. | Restore/unminimize window before sending input. |
| `ErrTargetNotForeground` | Window keyboard input under HID requires its top-level ancestor to be foreground. | Focus the target window before sending keyboard input. |
| `ErrDirectedScrollUnsupported` | `Window.Scroll` called under `BackendHID`. | Use global `ScrollWheel` for hardware-level scrolling. |
| `ErrDriverNotInstalled` | The Interception driver context could not be opened (HID only). | Prompt the user to install the driver, or keep the Message backend. |
| `ErrDLLLoadFailed` | The interception library is missing, its symbols are missing, or no mouse and no keyboard device was found. | Check `SetHIDLibraryPath`, the DLL, and the driver devices. This is not `ErrDriverNotInstalled`. |
| `ErrTargetChanged` | During `ProbeTarget`, the window still existed, but its process, thread, or a class name already read at the start changed. | Probe again. No partial report is returned. |
| `ErrInvalidBackend` | `NewInput` received a value other than `BackendMessage` or `BackendHID`. | Pass one of those two. `BackendMessage` is `0` and is legal. |
| `ErrInputInvalid` | The executor or facade did not come from `NewInput`, `ForWindow`, or `Desktop`. | Use the returned value. `ForWindow(nil)` fails later with `ErrWindowGone`. |
| `ErrUnsupportedKey` | Character cannot be mapped to a key. | Check input string encoding or use raw `KeyDown` for special keys. |
| `ErrPermissionDenied` | Operation blocked (e.g., UIPI). | Run your application as Administrator. |

Example of robust error handling:

```go
// SetBackend now fails fast if the driver or DLL is missing.
if err := winput.SetBackend(winput.BackendHID); err != nil {
    log.Printf("HID backend not available: %v. Falling back to Message backend.", err)
    // No need to explicitly set BackendMessage, as it is the default.
}

// All subsequent calls will use the successfully set backend.
if err := w.Click(100, 100); err != nil {
    log.Fatal(err)
}
```

## Advanced Usage

### 1. Handling High-DPI Monitors
Modern Windows scales applications. To ensure your `(100, 100)` click lands on the correct pixel:

```go
// Call this at program start
if err := winput.EnablePerMonitorDPI(); err != nil {
    log.Printf("DPI Awareness failed: %v", err)
}

// Check window specific DPI (96 is standard 100%)
dpi, _ := w.DPI()
fmt.Printf("Target Window DPI: %d (Scale: %.2f%%)\n", dpi, float64(dpi)/96.0*100)
```

### 2. HID Backend with Fallback
Use HID for games/anti-cheat, fallback to Message for standard apps.

```go
// Try to enable HID backend
if err := winput.SetBackend(winput.BackendHID); err != nil {
    log.Println("HID init failed, using default Message backend:", err)
    // No action needed, default is already BackendMessage
}

// Message types into w. HID returns ErrTargetNotForeground unless w's
// top-level window is already foreground.
w.Type("password")
```

### 3. Probes and a task-level executor
`ProbeTarget` and `ProbeHID` only report facts. They do not select a backend. `ProbeHID` can load the driver; a failure is still a status, and the global backend stays as it was.

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

`status` is one of `initialized`, `dll_unavailable`, `driver_unavailable`, `devices_unavailable`, or `unknown`. `PasteByClipboard` still uses `SetBackend`.

### 4. Key Mapping Details
`winput` maps runes to Scan Codes (Set 1).
- **Supported**: A-Z, 0-9, Common Symbols (`!`, `@`, `#`...), Space, Enter, Tab.
- **Auto-Shift**: `Type("A")` automatically sends `Shift Down` -> `a Down` -> `a Up` -> `Shift Up`.



## Comparison

| Feature        | winput (Go)                   | C# Interceptor Wrappers | Python winput (ctypes) |
| :------------- | :---------------------------- | :---------------------- | :--------------------- |
| **Backends**   | **Dual (HID + Message)**      | HID (Interception) Only | Message (User32) Only  |
| **API Style**  | Object-Oriented (`w.Click`)   | Low-level (`SendInput`) | Function-based         |
| **Dependency** | None (Default) / Driver (HID) | Driver Required         | None                   |
| **Safety**     | Explicit Errors               | Exceptions / Silent     | Silent / Return Codes  |
| **DPI Aware**  | ✅ Yes                         | ❌ Manual calc needed    | ❌ Manual calc needed   |

*   **vs Python winput**: Python's version is great for simple automation but lacks the kernel-level injection capability required for games or stubborn applications.
*   **vs C# Interceptor**: Most C# wrappers expose the raw driver API. `winput` abstracts this into high-level actions (Click, Type) and adds coordinate translation logic.



## Selection Guide: winput vs robotgo

| Feature | robotgo | winput |
| :--- | :--- | :--- |
| **Input Level** | OS Synthetic (`SendInput` / Events) | **Kernel Driver** (`Interception`) + OS Message |
| **Background Control** | ❌ Fails often | ✅ **Native Support** (PostMessage) |
| **Anti-Cheat / Protected Apps** | ❌ Blocked | ✅ **Bypasses most protections** (HID level) |
| **Coordinate System** | Implicit / Device-dependent | **Explicit Virtual Desktop** (DPI-aware) |
| **Visual Integration** | Basic | **Engineered for OpenCV/OCR** (Capture -> Map -> Input) |
| **Cross-Platform** | ✅ (Win/Mac/Linux) | ❌ (Windows Optimized) |
| **Use Case** | Quick scripts, simple GUI automation | **Engineering-grade automation**, Games, Electron, Background tasks |

**Summary**:
*   Use **robotgo** for quick, cross-platform scripts where reliability in edge cases (games, background) is not critical.
*   Use **winput** when you need **determinism**, **driver-level control**, or need to interact with **background windows**, **games**, or **Electron apps** that ignore standard input injection.



## License

MIT
