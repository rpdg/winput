# winput

**winput** is a lightweight, high-performance Go library for Windows background input automation.

It provides a unified, window-centric API that abstracts the underlying input mechanism, allowing seamless switching between standard Window Messages (`PostMessage`) and kernel-level injection (`Interception` driver).

## Features

*   **Window-Centric API**: Operations are performed on `Window` objects, not raw HWNDs.
*   **Background Input**: 
    *   **Message Backend**: Sends inputs directly to window message queues. Works without window focus or mouse cursor movement.
    *   **HID Backend**: Uses the [Interception](https://github.com/oblitum/Interception) driver to simulate hardware input at the kernel level.
*   **Coordinate Management**:
    *   Unified **Client Coordinate** system for all APIs.
    *   Built-in `ScreenToClient` / `ClientToScreen` conversion.
    *   **DPI Awareness**: Helpers for Per-Monitor DPI scaling.
*   **Safety & Reliability**:
    *   Explicit error returns (no silent failures).
    *   Type-safe Key definitions.

## Installation

```bash
go get github.com/rpdg/winput
```

## Quick Start

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
}
```

## Error Handling

winput avoids silent failures. Common errors you should handle:

| Error Variable | Description | Handling |
| :--- | :--- | :--- |
| `ErrWindowNotFound` | Window not found by Title/Class/PID. | Check if the app is running or use `FindByClass` as fallback. |
| `ErrDriverNotInstalled` | Interception driver missing (HID mode only). | Prompt user to install the driver or fallback to Message backend. |
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