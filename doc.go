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
//   - BackendMessage (Default): Uses PostMessage for background input. Does not require focus.
//   - BackendHID: Uses the Interception driver for kernel-level simulation (requires driver installation).
//     Useful for applications that block standard Windows messages.
//
// 3. Coordinate System:
// All API calls accept window-relative (client) coordinates. The library handles:
//   - Screen <-> Client conversion
//   - DPI scaling (Per-Monitor v2 awareness)
//
// 4. Explicit Error Handling:
// No silent failures. Defined errors (ErrWindowNotFound, ErrBackendUnavailable, etc.)
// allow for robust control flow.
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
//  w.Click(100, 100)       // Click at client (100, 100)
//  w.Type("Hello World")   // Background typing
//  w.Press(winput.KeyEnter)
//
//  // 4. Switch to HID backend (if needed)
//  // winput.SetBackend(winput.BackendHID)
//  // w.Move(50, 50) // Now moves the actual mouse cursor to (50,50) relative to the window
//
package winput