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
	ProcIsWindow                 = user32.NewProc("IsWindow")
	ProcIsWindowVisible          = user32.NewProc("IsWindowVisible")
	ProcIsIconic                 = user32.NewProc("IsIconic")
	ProcGetClassNameW            = user32.NewProc("GetClassNameW")

	ProcScreenToClient    = user32.NewProc("ScreenToClient")
	ProcClientToScreen    = user32.NewProc("ClientToScreen")
	ProcGetClientRect     = user32.NewProc("GetClientRect")
	ProcGetCursorPos      = user32.NewProc("GetCursorPos")
	ProcMonitorFromPoint  = user32.NewProc("MonitorFromPoint")
	ProcMonitorFromWindow = user32.NewProc("MonitorFromWindow")

	ProcGetDpiForWindow           = user32.NewProc("GetDpiForWindow") // Win10+
	ProcSetProcessDpiAwarenessCtx = user32.NewProc("SetProcessDpiAwarenessContext")

	ProcGetDpiForMonitor = shcore.NewProc("GetDpiForMonitor")

	ProcGetDC         = user32.NewProc("GetDC")
	ProcReleaseDC     = user32.NewProc("ReleaseDC")
	ProcGetDeviceCaps = gdi32.NewProc("GetDeviceCaps")

	ProcPostMessageW   = user32.NewProc("PostMessageW")
	ProcMapVirtualKeyW = user32.NewProc("MapVirtualKeyW")

	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	ProcCreateToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	ProcProcess32First           = kernel32.NewProc("Process32FirstW")
	ProcProcess32Next            = kernel32.NewProc("Process32NextW")
	ProcCloseHandle              = kernel32.NewProc("CloseHandle")
)
