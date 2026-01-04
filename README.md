# winput

**winput** is a lightweight, high-performance Go library for Windows background input automation.

It provides a unified, window-centric API that abstracts the underlying input mechanism, allowing seamless switching between standard Window Messages (`PostMessage`) and kernel-level injection (`Interception` driver).

## Features

*   **Pure Go (No CGO)**: Uses dynamic DLL loading. No GCC required for compilation.
*   **Window-Centric API**: Operations are performed on `Window` objects, not raw HWNDs.
*   **Background Input**: 
    *   **Message Backend**: Sends inputs directly to window message queues. Works without window focus or mouse cursor movement.
    *   **HID Backend**: Uses the [Interception](https://github.com/oblitum/Interception) driver to simulate hardware input at the kernel level.
*   **Coordinate Management**:
    *   Unified **Client Coordinate** system for all APIs.
    *   Built-in `ScreenToClient` / `ClientToScreen` conversion.
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

	// 1. Capture the entire virtual desktop (all monitors)
	img, err := screen.CaptureVirtualDesktop()
	if err != nil {
		panic(err)
	}
	// img is a standard *image.RGBA, ready for OpenCV/GoCV

	// 2. Perform your CV matching here (pseudo-code)
	// matchX, matchY := yourCVLib.Match(img, template)

	// 3. Convert image coordinates to virtual desktop coordinates
	targetX, targetY := screen.ImageToVirtual(int32(matchX), int32(matchY))

	// 4. Move and Click
	winput.MoveMouseTo(targetX, targetY)
	winput.ClickMouseAt(targetX, targetY)
}
```

## API Reference

## Backend Limitations & Permissions

### Message Backend
*   **Mechanism**: Uses `PostMessageW`.
*   **Pros**: No focus required, no mouse movement, works in background.
*   **Cons**: 
    *   **Modifier Keys**: `PostMessage` does **not** update global keyboard state. Apps checking `GetKeyState` (e.g. for Ctrl+C) might fail.
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
    *   **Focus**: Usually requires the window to be active/foreground.

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
    winput.SetHIDLibraryPath("libs/interception.dll")
    winput.SetBackend(winput.BackendHID)
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

## Quick Start (Code Snippet)

```go
package main

import (
	"log"
	"github.com/rpdg/winput"
)

func main() {
	// 1. Find target window
	w, err := winput.FindByTitle("Untitled - Notepad")
	if err != nil {
		log.Fatal(err)
	}

	// 2. Click (Left Button)
	if err := w.Click(100, 100); err != nil {
		log.Fatal(err)
	}

	// 3. Type text
	w.Type("Hello World")
	w.Press(winput.KeyEnter)

	// 4. Global Input (Target independent)
	winput.Type("Hello Electron!")
	winput.Press(winput.KeyEnter)

	// 5. Using winput/screen (Boundary query)
	// import "github.com/rpdg/winput/screen"
	bounds := screen.VirtualBounds()
	fmt.Printf("Virtual Desktop Bounds: %d, %d\n", bounds.Right, bounds.Bottom)
}
```

## Error Handling

winput avoids silent failures. Common errors you should handle:

| Error Variable | Description | Handling |
| :--- | :--- | :--- |
| `ErrWindowNotFound` | Window not found by Title/Class/PID. | Check if the app is running or use `FindByClass` as fallback. |
| `ErrDriverNotInstalled` | Interception driver missing (HID mode only). | Prompt user to install the driver or fallback to Message backend. |
| `ErrDLLLoadFailed` | `interception.dll` not found or invalid. | Check DLL path (`SetHIDLibraryPath`) or installation. |
| `ErrUnsupportedKey` | Character cannot be mapped to a key. | Check input string encoding or use raw `KeyDown` for special keys. |
| `ErrPermissionDenied` | Operation blocked (e.g., UIPI). | Run your application as Administrator. |

Example of robust error handling:

```go
if err := winput.SetBackend(winput.BackendHID); err != nil {
    // This won't fail immediately, but checkBackend will fail on first action
}

err := w.Click(100, 100)
if errors.Is(err, winput.ErrDriverNotInstalled) {
    log.Println("HID driver missing, falling back to Message backend...")
    winput.SetBackend(winput.BackendMessage)
    w.Click(100, 100) // Retry
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
fmt.Printf("Target Window DPI: %d (Scale: %.2f%%)
", dpi, float64(dpi)/96.0*100)
```

### 2. HID Backend with Fallback
Use HID for games/anti-cheat, fallback to Message for standard apps.

```go
winput.SetBackend(winput.BackendHID)
err := w.Type("password")
if err != nil {
    // If HID fails (e.g. driver not installed), switch back
    winput.SetBackend(winput.BackendMessage)
    w.Type("password")
}
```

### 3. Key Mapping Details
`winput` maps runes to Scan Codes (Set 1).
- **Supported**: A-Z, 0-9, Common Symbols (`!`, `@`, `#`...), Space, Enter, Tab.
- **Auto-Shift**: `Type("A")` automatically sends `Shift Down` -> `a Down` -> `a Up` -> `Shift Up`.

## Comparison

| Feature | winput (Go) | C# Interceptor Wrappers | Python winput (ctypes) |
| :--- | :--- | :--- | :--- |
| **Backends** | **Dual (HID + Message)** | HID (Interception) Only | Message (User32) Only |
| **API Style** | Object-Oriented (`w.Click`) | Low-level (`SendInput`) | Function-based |
| **Dependency** | None (Default) / Driver (HID) | Driver Required | None |
| **Safety** | Explicit Errors | Exceptions / Silent | Silent / Return Codes |
| **DPI Aware** | ✅ Yes | ❌ Manual calc needed | ❌ Manual calc needed |

*   **vs Python winput**: Python's version is great for simple automation but lacks the kernel-level injection capability required for games or stubborn applications.
*   **vs C# Interceptor**: Most C# wrappers expose the raw driver API. `winput` abstracts this into high-level actions (Click, Type) and adds coordinate translation logic.

## License

MIT