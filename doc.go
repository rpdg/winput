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
// 3. Coordinate System & Screen Management:
//   - Window-Centric: Operations on *Window use client coordinates.
//   - Global Input: MoveMouseTo/ClickMouseAt/Type for absolute virtual desktop coordinates (useful for Electron/Games).
//   - Child Windows: FindChildByClass for targeting specific controls (e.g. "Edit" in Notepad).
//   - Screen Package: winput/screen helpers for querying monitor bounds and virtual desktop geometry.
//   - DPI Awareness: Per-Monitor v2 support for accurate mapping.
//
// 4. Intelligent Keyboard Input:
//   - Type(string) automatically handles Shift modifiers for uppercase letters and symbols.
//   - Uses Scan Codes (Set 1) for maximum compatibility with low-level hooks and games.
//
// 5. Robust Error Handling:
// Defines standard errors like ErrWindowNotFound, ErrDriverNotInstalled, ErrPostMessageFailed.
// It follows an "explicit failure" principle, where backend initialization errors are reported
// on the first attempted action.
//
// 6. Thread Safety:
// All public input methods are thread-safe and serialized using an internal mutex. This prevents state pollution
// (e.g., mixing Shift states from concurrent operations) and race conditions when switching backends.
//
// Example:
//
//	 // For complete examples, see cmd/example/
//	 // - basic_message: Background automation for standard apps
//	 // - global_vision: Electron/Game automation using screen coordinates
//	 // - advanced_hid: Hardware level simulation
//
//		// 1. Find the window
//		w, err := winput.FindByTitle("Untitled - Notepad")
//		if err != nil {
//		    log.Fatal(winput.ErrWindowNotFound)
//		}
//
//		// 2. Setup DPI awareness (optional but recommended)
//		winput.EnablePerMonitorDPI()
//
//		// 3. Perform actions (using default Message backend)
//		w.Click(100, 100)       // Left click
//		w.ClickRight(100, 100)  // Right click
//		w.Scroll(100, 100, 120) // Vertical scroll
//
//		w.Type("Hello World!")  // Automatically handles Shift for 'H', 'W', and '!'
//		w.Press(winput.KeyEnter)
//
//		// 4. Global Input (Visual Automation / Electron)
//		// winput.MoveMouseTo(1920, 500)
//		// winput.ClickMouseAt(1920, 500)
//
//		// 5. Switch to HID backend for hardware-level simulation
//		// winput.SetHIDLibraryPath("libs/interception.dll")
//		// winput.SetBackend(winput.BackendHID)
package winput
