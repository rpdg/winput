package window

import (
	"fmt"
	"unsafe"
)

// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 is (HANDLE)(-4)
var dpiAwarenessPerMonitorV2 = ^uintptr(3)

func EnablePerMonitorDPI() error {
	if ProcSetProcessDpiAwarenessCtx.Find() != nil {
		return fmt.Errorf("SetProcessDpiAwarenessContext not found")
	}
	r, _, _ := ProcSetProcessDpiAwarenessCtx.Call(dpiAwarenessPerMonitorV2)
	if r == 0 {
		return fmt.Errorf("SetProcessDpiAwarenessContext failed")
	}
	return nil
}

func GetDPI(hwnd uintptr) (uint32, uint32, error) {
	// 1. Try GetDpiForWindow (Win10+) - returns same value for X and Y usually
	if ProcGetDpiForWindow.Find() == nil {
		r, _, _ := ProcGetDpiForWindow.Call(hwnd)
		if r != 0 {
			return uint32(r), uint32(r), nil
		}
	}
	
	// 2. Try GetDpiForMonitor (Win8.1+)
	hMon := MonitorFromWindow(hwnd)
	if hMon != 0 {
		dx, dy, err := GetDpiForMonitor(hMon)
		if err == nil {
			return dx, dy, nil
		}
	}
	
	// 3. Fallback: GetDeviceCaps (Win7/Legacy)
	hdc, _, _ := ProcGetDC.Call(hwnd)
	if hdc != 0 {
		defer ProcReleaseDC.Call(hwnd, hdc)
		// LOGPIXELSX = 88, LOGPIXELSY = 90
		dpix, _, _ := ProcGetDeviceCaps.Call(hdc, 88)
		dpiy, _, _ := ProcGetDeviceCaps.Call(hdc, 90)
		if dpix > 0 && dpiy > 0 {
			return uint32(dpix), uint32(dpiy), nil
		}
	}
	
	return 96, 96, fmt.Errorf("cannot determine DPI")
}

func MonitorFromWindow(hwnd uintptr) uintptr {
	const MONITOR_DEFAULTTONEAREST = 2
	r, _, _ := ProcMonitorFromWindow.Call(hwnd, MONITOR_DEFAULTTONEAREST)
	return r
}

func GetDpiForMonitor(hmonitor uintptr) (dpiX, dpiY uint32, err error) {
	if ProcGetDpiForMonitor.Find() != nil {
		return 96, 96, fmt.Errorf("GetDpiForMonitor not found")
	}
	var dx, dy uint32
	// MDT_EFFECTIVE_DPI = 0
	r, _, _ := ProcGetDpiForMonitor.Call(hmonitor, 0, uintptr(unsafe.Pointer(&dx)), uintptr(unsafe.Pointer(&dy)))
	if r != 0 {
		return 96, 96, fmt.Errorf("GetDpiForMonitor failed")
	}
	return dx, dy, nil
}