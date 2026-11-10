package window

import (
	"fmt"
	"unsafe"
)

// DPI Awareness Contexts (Pseudo-Handles)
// Win10 1607+
// Using ^uintptr(0) to correctly represent -1 on both 32-bit and 64-bit systems.
const (
	DPI_AWARENESS_CONTEXT_UNAWARE              = ^uintptr(0) // -1
	DPI_AWARENESS_CONTEXT_SYSTEM_AWARE         = ^uintptr(1) // -2
	DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE    = ^uintptr(2) // -3
	DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = ^uintptr(3) // -4
	DPI_AWARENESS_CONTEXT_UNAWARE_GDISCALED    = ^uintptr(4) // -5
)

// EnablePerMonitorDPI attempts to set the process to Per-Monitor DPI Aware (V2).
// It falls back to V1 or System Aware on older systems if V2 is unavailable.
func EnablePerMonitorDPI() error {
	// Try SetProcessDpiAwarenessContext (Win10 1607+)
	if err := ProcSetProcessDpiAwarenessCtx.Find(); err == nil {
		// Prefer V2
		r, _, _ := ProcSetProcessDpiAwarenessCtx.Call(DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2)
		if r != 0 { // S_OK is not 0 for this API? Wait, check MSDN.
			// MSDN: "If the function succeeds, the return value is TRUE." -> Non-zero.
			// Wait, SetProcessDpiAwarenessContext returns BOOL.
			// If it succeeds, it returns non-zero.
			return nil
		}
		// Fallback to V1
		r, _, _ = ProcSetProcessDpiAwarenessCtx.Call(DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE)
		if r != 0 {
			return nil
		}
	}

	// Fallback for older Windows (8.1/10 early) - Shcore.dll
	// SetProcessDpiAwareness(PROCESS_PER_MONITOR_DPI_AWARE = 2)
	procSetProcessDpiAwareness := shcore.NewProc("SetProcessDpiAwareness")
	if procSetProcessDpiAwareness.Find() == nil {
		r, _, _ := procSetProcessDpiAwareness.Call(2)
		if r == 0 { // S_OK = 0
			return nil
		}
	}

	// Fallback for Vista/7/8 - User32.dll
	// SetProcessDPIAware()
	procSetProcessDPIAware := user32.NewProc("SetProcessDPIAware")
	if procSetProcessDPIAware.Find() == nil {
		r, _, _ := procSetProcessDPIAware.Call()
		if r != 0 { // BOOL, non-zero is success
			return nil
		}
	}

	return fmt.Errorf("failed to set DPI awareness on this Windows version")
}

// GetDPI returns the DPI for the specified window.
// It tries to use GetDpiForWindow (Win10 1607+), falling back to System DPI.
func GetDPI(hwnd uintptr) (uint32, uint32, error) {
	// Try GetDpiForWindow (Win10 1607+)
	if err := ProcGetDpiForWindow.Find(); err == nil {
		dpi, _, _ := ProcGetDpiForWindow.Call(hwnd)
		if dpi != 0 {
			return uint32(dpi), uint32(dpi), nil
		}
	}

	// Fallback: GetDC -> GetDeviceCaps
	// This returns the System DPI (or Per-Monitor if the process is aware, but less reliable for mixed setups)
	hdc, _, _ := ProcGetDC.Call(hwnd)
	if hdc == 0 {
		return 96, 96, fmt.Errorf("GetDC failed")
	}
	defer ProcReleaseDC.Call(hwnd, hdc)

	const (
		LOGPIXELSX = 88
		LOGPIXELSY = 90
	)

	dpiX, _, _ := ProcGetDeviceCaps.Call(hdc, LOGPIXELSX)
	dpiY, _, _ := ProcGetDeviceCaps.Call(hdc, LOGPIXELSY)

	if dpiX == 0 {
		dpiX = 96
	}
	if dpiY == 0 {
		dpiY = 96
	}

	return uint32(dpiX), uint32(dpiY), nil
}

// IsPerMonitorDPIAware reports whether the current process is per-monitor
// DPI aware (V1 or V2). Screen capture checks the thread when those APIs
// exist, and falls back to this function when they do not.
//
// On Windows 10, GetDpiAwarenessContextForProcess(NULL) and
// GetProcessDpiAwareness follow the calling thread once
// SetThreadDpiAwarenessContext has changed it. A per-monitor process can
// therefore look system-aware to this call. Call it from a thread that still
// has the process default when that distinction matters.
func IsPerMonitorDPIAware() bool {
	// Windows 10 1803+: GetDpiAwarenessContextForProcess(NULL) is the current
	// process. Pass 0 for NULL. V1 and V2 are both per-monitor.
	if err := ProcGetDpiAwarenessContextForProcess.Find(); err == nil {
		if err := ProcAreDpiAwarenessContextsEqual.Find(); err == nil {
			ctx, _, _ := ProcGetDpiAwarenessContextForProcess.Call(0)
			if ctx != 0 {
				// Check for V2
				r1, _, _ := ProcAreDpiAwarenessContextsEqual.Call(ctx, DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2)
				if r1 != 0 {
					return true
				}
				// Check for V1
				r2, _, _ := ProcAreDpiAwarenessContextsEqual.Call(ctx, DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE)
				if r2 != 0 {
					return true
				}
			}
		}
	}

	// 2. Try Windows 8.1 API (Shcore.dll)
	if err := ProcGetProcessDpiAwareness.Find(); err == nil {
		var awareness int32
		// 0 = current process (NULL handle)
		r, _, _ := ProcGetProcessDpiAwareness.Call(0, uintptr(unsafe.Pointer(&awareness)))
		if r == 0 { // S_OK
			// PROCESS_PER_MONITOR_DPI_AWARE = 2
			return awareness == 2
		}
	}

	// 3. Try Vista/Win7 API (User32.dll)
	// Note: This only returns if the process is "Aware" (System Aware),
	// it doesn't distinguish Per-Monitor. But for older systems, this is the best we can check.
	if err := ProcIsProcessDPIAware.Find(); err == nil {
		r, _, _ := ProcIsProcessDPIAware.Call()
		return r != 0
	}

	return false
}
