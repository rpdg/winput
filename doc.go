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
//     for ordinary input and is ideal for non-intrusive automation, but window hotkeys with
//     modifiers may temporarily switch to foreground keyboard events for reliability.
//   - BackendHID: Uses the Interception driver for kernel-level simulation (requires driver installation).
//     This mode simulates hardware-level input, complete with human-like mouse movement trajectories
//     and jitter. Supports a custom DLL path via SetHIDLibraryPath. Call that before the first
//     successful ProbeHID, NewInput(BackendHID), or SetBackend(BackendHID). That success freezes
//     the DLL path for the rest of the process.
//
// 3. Coordinate System & Screen Management:
//   - Window-Centric: Operations on *Window use client coordinates.
//   - Global Input: MoveMouseTo/ClickMouseAt/Type for absolute virtual desktop coordinates (useful for Electron/Games).
//   - Child Windows: FindChildByClass for targeting specific controls (e.g. "Edit" in Notepad).
//   - Text Reading: Text() for standard Win32 controls, Value() for best-effort reads with UI Automation fallback.
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
// on the first attempted action. ErrDLLLoadFailed covers a missing library, missing symbols,
// and no mouse and no keyboard device. Only a failure to open the driver context is
// ErrDriverNotInstalled. ProbeTarget returns ErrWindowGone or ErrTargetChanged with no report.
// NewInput returns ErrInvalidBackend for any value other than BackendMessage or BackendHID,
// before it touches HID. A zero Input or a facade that did not come from ForWindow or Desktop
// returns ErrInputInvalid. ForWindow(nil) returns ErrWindowGone when a method is called.
// A window key-down on BackendHID returns ErrTargetNotForeground, and sends nothing,
// when that window's top-level ancestor is not the foreground window. Window.KeyUp
// still sends, so a key that is already down can be released.
//
// 6. Thread Safety:
// All public input methods are thread-safe and serialized using an internal mutex. This prevents state pollution
// (e.g., mixing Shift states from concurrent operations) and race conditions when switching backends.
// The mutex covers one public input call. A sequence such as MouseDown, Move, and MouseUp spans multiple
// calls and is not atomic; callers that need that atomicity must serialize the whole sequence.
//
// 7. Facts and task-level executors:
//   - ProbeTarget reports an executable file name, the target window's own class, the integrity
//     relation (target_higher, equal, target_lower, unknown), and caller UIAccess (enabled,
//     disabled, unknown). Warnings are only uipi_likely_blocked, integrity_unknown, and
//     uiaccess_unknown. It does not choose a backend, initialize HID, or change SetBackend.
//   - ProbeHID returns initialized, dll_unavailable, driver_unavailable, devices_unavailable, or
//     unknown. A failed initialization is still a status. initialized means only that this process
//     finished HID initialization. It does not change the global backend, and it does not mean a
//     window will accept the input.
//   - NewInput binds BackendMessage or BackendHID. Window calls use WindowInput. Screen and
//     windowless calls use DesktopInput. A later SetBackend does not change that executor.
//     The package-level PasteByClipboard keeps the global backend; DesktopInput.PasteByClipboard
//     uses the executor's fixed backend. Window search, text reads, cursor position, and DPI
//     stay outside Input.
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
//		w.Scroll(100, 100, 120) // Vertical scroll (message backend only)
//
//		// Window.Scroll posts WM_MOUSEWHEEL to the target window. A zero
//		// delta is a no-op, and any other delta must fit in int16. Under
//		// BackendHID, delta is not validated and the call returns
//		// ErrDirectedScrollUnsupported, because a hardware wheel event
//		// cannot be addressed to a specific window. Use ScrollWheel for
//		// hardware-level whole-step scrolling.
//
//		w.Type("Hello World!")  // Automatically handles Shift for 'H', 'W', and '!'
//		w.Press(winput.KeyEnter)
//
//		// 4. Global Input (Visual Automation / Electron)
//		// winput.MoveMouseTo(1920, 500)
//		// winput.ClickMouseAt(1920, 500)
//
//		// 5. Switch to HID backend for hardware-level simulation.
//		// Set the DLL path before the first successful ProbeHID, NewInput(BackendHID),
//		// or SetBackend(BackendHID).
//		// winput.SetHIDLibraryPath("libs/interception.dll")
//		// winput.SetBackend(winput.BackendHID)
//		//
//		// 6. Facts, then a task-level executor. Neither probe selects a backend.
//		// report, err := winput.ProbeTarget(w)
//		// status := winput.ProbeHID()
//		// input, err := winput.NewInput(winput.BackendMessage)
//		// win, err := input.ForWindow(w)
//		// win.Type("123456")
package winput
