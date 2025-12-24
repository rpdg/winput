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
    *   Type-safe Key definitions (avoiding raw scan code usage).

## Installation

```bash
go get github.com/yourusername/winput
```

### Prerequisites for HID Backend
If you intend to use `BackendHID` (Interception), you must:
1.  Install the **Interception driver** (run `install-interception.exe` from the official release).
2.  Ensure `interception.dll` is in your application's working directory or system PATH.
3.  Ensure CGO is enabled (requires a C compiler like MinGW).

> **Note**: The default `BackendMessage` does **not** require drivers or CGO (runtime-wise, though this library uses CGO for linking the optional HID backend).

## Usage

### Basic Example (Background Message)

```go
package main

import (
	"log"
	"winput"
)

func main() {
	// 1. Find target window
	w, err := winput.FindByTitle("Untitled - Notepad")
	if err != nil {
		log.Fatal("Window not found:", err)
	}

	// 2. Click at (100, 100) inside the window
	// This does not move the physical mouse cursor.
	if err := w.Click(100, 100); err != nil {
		log.Fatal(err)
	}

	// 3. Type text
	w.Type("Hello Background World")
	w.Press(winput.KeyEnter)
}
```

### Switching to HID Backend

Use this mode if the target application blocks `PostMessage` or if you need to simulate physical hardware behavior (e.g., games, anti-cheat protected apps).

```go
func main() {
    w, _ := winput.FindByClass("Notepad")

    // Switch global backend to HID
    // Note: This requires the Interception driver to be installed.
    // Initialization errors will be returned on the first action.
    winput.SetBackend(winput.BackendHID)

    // This will now physically move the mouse cursor to (100, 100) relative to the window
    err := w.Click(100, 100)
    if err != nil {
        log.Fatalf("HID Input failed (driver missing?): %v", err)
    }
}
```

## Architecture

### Package Structure
```
winput/
├── window/      # Win32 Window & DPI APIs
├── mouse/       # PostMessage mouse implementation
├── keyboard/    # PostMessage keyboard implementation
├── hid/         # Interception driver wrapper & logic
│   └── interception/ # Low-level CGO bindings
└── winput.go    # Public API & Backend switching
```

### Design Principles
1.  **Coordinate Consistency**: The API *always* accepts window-client coordinates. The backend handles the translation to screen coordinates if necessary (e.g., for HID injection).
2.  **Explicit Failure**: If an operation cannot be performed (e.g., window closed, backend unavailable), it returns a specific error.
3.  **Zero-Cost Abstraction**: The default message backend is lightweight and pure Go (syscalls). The HID backend is loaded only when requested.

## License

MIT
