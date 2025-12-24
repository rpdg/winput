// Package winput provides a Windows input automation library focused on background operation.
// It abstracts input injection to support multiple backends while maintaining a consistent,
// object-centric API.
//
// Key Features:
//
// 1. Window-Centric Design:
// All operations are methods on a *Window object. The library handles HWND management,
// thread attachment, and coordinate translation internally.
//
// 2. Dual Input Backends:
//   - BackendMessage (Default): Uses PostMessage for background input. It does not require focus
//     and is ideal for non-intrusive automation.
//   - BackendHID: Uses the Interception driver for kernel-level simulation (requires driver installation).
//     This mode simulates hardware-level input, complete with human-like mouse movement trajectories
//     and jitter, making it indistinguishable from physical input.
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
// Defines standard errors like ErrWindowNotFound and ErrDriverNotInstalled. It follows
// an "explicit failure" principle, where backend initialization errors are reported
// on the first attempted action.
//
// Example:
//
//  // 1. Find the window
//  w, err := winput.FindByTitle("Untitled - Notepad")
//  if err != nil {
//      log.Fatal(winput.ErrWindowNotFound)
//  }
//
//  // 2. Setup DPI awareness (optional but recommended)
//  winput.EnablePerMonitorDPI()
//
//  // 3. Perform actions (using default Message backend)
//  w.Click(100, 100)       // Left click
//  w.ClickRight(100, 100)  // Right click
//  w.Scroll(100, 100, 120) // Vertical scroll
//
//  w.Type("Hello World!")  // Automatically handles Shift for 'H', 'W', and '!'
//  w.Press(winput.KeyEnter)
//
//  // 4. Switch to HID backend for hardware-level simulation
//  // winput.SetBackend(winput.BackendHID)
//  // err := w.Move(50, 50) // Moves physical cursor with human-like trajectory
//  // if errors.Is(err, winput.ErrDriverNotInstalled) { ... }
//
package winput