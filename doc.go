// Package winput provides a Windows input automation library focused on background operation.
// It abstracts input injection to support multiple backends while maintaining a consistent,
// object-centric API.
//
// Key Features:
//
// 1. Pure Go & No CGO:
// This library uses dynamic DLL loading (syscall.LoadLibrary) and does not require a CGO
// compiler environment (GCC) for building.
//
// 2. Dual Input Backends:
//   - BackendMessage (Default): Uses PostMessage for background input. It does not require focus
//     and is ideal for non-intrusive automation.
//   - BackendHID: Uses the Interception driver for kernel-level simulation (requires driver installation).
//     This mode simulates hardware-level input, complete with human-like mouse movement trajectories
//     and jitter. Supports custom DLL path via SetHIDLibraryPath.
//
// 3. Coordinate System & DPI:
// All API calls accept window-relative (client) coordinates. The library handles:
//   - Screen <-> Client conversion.
//   - Multi-level DPI discovery (Per-Monitor v2, falling back to GDI DeviceCaps for legacy systems).
//
// 4. Intelligent Keyboard Input:
//   - Type(string) automatically handles Shift modifiers for uppercase letters and symbols.
//   - Uses Scan Codes (Set 1) for maximum compatibility with low-level hooks and games.
//
// 5. Robust Error Handling:
// Defines standard errors like ErrWindowNotFound, ErrDriverNotInstalled, ErrWindowNotVisible, and ErrDLLLoadFailed.
// It follows an "explicit failure" principle, where backend initialization errors are reported
// on the first attempted action.
//
// Example:
//
//	// 1. Find the window
//	w, err := winput.FindByTitle("Untitled - Notepad")
//	if err != nil {
//	    log.Fatal(winput.ErrWindowNotFound)
//	}
//
//	// 2. Setup DPI awareness (optional but recommended)
//	winput.EnablePerMonitorDPI()
//
//	// 3. Perform actions (using default Message backend)
//	w.Click(100, 100)       // Left click
//	w.ClickRight(100, 100)  // Right click
//	w.Scroll(100, 100, 120) // Vertical scroll
//
//	w.Type("Hello World!")  // Automatically handles Shift for 'H', 'W', and '!'
//	w.Press(winput.KeyEnter)
//
//	// 4. Switch to HID backend for hardware-level simulation
//	// winput.SetHIDLibraryPath("libs/interception.dll")
//	// winput.SetBackend(winput.BackendHID)
//	// err := w.MoveRel(10, 10) // Moves physical cursor
//	// if errors.Is(err, winput.ErrDriverNotInstalled) { ... }
package winput
