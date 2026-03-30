package window

import (
	"syscall"
)

var (
	user32 = syscall.NewLazyDLL("user32.dll")
	shcore = syscall.NewLazyDLL("shcore.dll")
	gdi32  = syscall.NewLazyDLL("gdi32.dll")

	ProcFindWindowW              = user32.NewProc("FindWindowW")
	ProcFindWindowExW            = user32.NewProc("FindWindowExW")
	ProcGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	ProcEnumWindows              = user32.NewProc("EnumWindows")
	ProcSendMessageW             = user32.NewProc("SendMessageW")
	ProcSendMessageTimeoutW      = user32.NewProc("SendMessageTimeoutW")
	ProcGetWindowTextW           = user32.NewProc("GetWindowTextW")
	ProcGetWindowTextLengthW     = user32.NewProc("GetWindowTextLengthW")
	ProcIsWindow                 = user32.NewProc("IsWindow")
	ProcIsWindowVisible          = user32.NewProc("IsWindowVisible")
	ProcIsIconic                 = user32.NewProc("IsIconic")
	ProcGetClassNameW            = user32.NewProc("GetClassNameW")

	ProcScreenToClient      = user32.NewProc("ScreenToClient")
	ProcClientToScreen      = user32.NewProc("ClientToScreen")
	ProcGetClientRect       = user32.NewProc("GetClientRect")
	ProcGetCursorPos        = user32.NewProc("GetCursorPos")
	ProcSetCursorPos        = user32.NewProc("SetCursorPos")
	ProcMouseEvent          = user32.NewProc("mouse_event")
	ProcKeybdEvent          = user32.NewProc("keybd_event")
	ProcSendInput           = user32.NewProc("SendInput")
	ProcMonitorFromPoint    = user32.NewProc("MonitorFromPoint")
	ProcMonitorFromWindow   = user32.NewProc("MonitorFromWindow")
	ProcEnumDisplayMonitors = user32.NewProc("EnumDisplayMonitors")
	ProcGetMonitorInfoW     = user32.NewProc("GetMonitorInfoW")
	ProcGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	ProcGetDoubleClickTime  = user32.NewProc("GetDoubleClickTime")

	// DPI Awareness (Win10 1607+)
	ProcGetDpiForWindow              = user32.NewProc("GetDpiForWindow")
	ProcSetProcessDpiAwarenessCtx    = user32.NewProc("SetProcessDpiAwarenessContext")
	ProcGetProcessDpiAwarenessCtx    = user32.NewProc("GetProcessDpiAwarenessContext")
	ProcAreDpiAwarenessContextsEqual = user32.NewProc("AreDpiAwarenessContextsEqual")
	ProcIsProcessDPIAware            = user32.NewProc("IsProcessDPIAware")

	ProcGetDpiForMonitor       = shcore.NewProc("GetDpiForMonitor")
	ProcGetProcessDpiAwareness = shcore.NewProc("GetProcessDpiAwareness")

	ProcGetDC     = user32.NewProc("GetDC")
	ProcReleaseDC = user32.NewProc("ReleaseDC")

	// GDI Functions for Capture
	ProcGetDeviceCaps      = gdi32.NewProc("GetDeviceCaps")
	ProcCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	ProcCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	ProcSelectObject       = gdi32.NewProc("SelectObject")
	ProcDeleteObject       = gdi32.NewProc("DeleteObject")
	ProcDeleteDC           = gdi32.NewProc("DeleteDC")
	ProcBitBlt             = gdi32.NewProc("BitBlt")

	ProcPostMessageW   = user32.NewProc("PostMessageW")
	ProcMapVirtualKeyW = user32.NewProc("MapVirtualKeyW")

	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	ProcCreateToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	ProcProcess32First           = kernel32.NewProc("Process32FirstW")
	ProcProcess32Next            = kernel32.NewProc("Process32NextW")
	ProcCloseHandle              = kernel32.NewProc("CloseHandle")
)
