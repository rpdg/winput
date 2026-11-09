package window

import "syscall"

// GA_ROOT asks GetAncestor for the top-level window that is not itself an
// owned window's owner. A top-level window is its own root. A WS_CHILD is not
// what GetForegroundWindow returns.
const gaRoot = 2

// AncestorRoot returns the top-level ancestor of hwnd.
// An invalid hwnd returns 0. GA_ROOTOWNER is intentionally not used: an owned
// dialog and its owner are different foreground identities.
func AncestorRoot(hwnd uintptr) uintptr {
	if hwnd == 0 {
		return 0
	}
	root, _, _ := ProcGetAncestor.Call(hwnd, gaRoot)
	return root
}

// GetForegroundWindow returns the current foreground window handle.
func GetForegroundWindow() uintptr {
	r, _, _ := ProcGetForegroundWindow.Call()
	return r
}

// SetForegroundWindow brings the specified window to the foreground.
func SetForegroundWindow(hwnd uintptr) error {
	r, _, e := ProcSetForegroundWindow.Call(hwnd)
	if r != 0 {
		return nil
	}
	if errno, ok := e.(syscall.Errno); ok && errno != 0 {
		return errno
	}
	return syscall.EINVAL
}

// ForceSetForegroundWindow brings hwnd to the foreground, bypassing the
// Win10/11 ASFW restriction by temporarily attaching the calling thread's
// input queue to the current foreground window's thread. Falls back to a
// plain SetForegroundWindow if the AttachThreadInput trick cannot be set up.
func ForceSetForegroundWindow(hwnd uintptr) error {
	if err := SetForegroundWindow(hwnd); err == nil {
		return nil
	}

	fg := GetForegroundWindow()
	if fg == 0 || fg == hwnd {
		return SetForegroundWindow(hwnd)
	}

	fgTid, _, _ := ProcGetWindowThreadProcessId.Call(fg, 0)
	myTid, _, _ := ProcGetCurrentThreadId.Call()
	if fgTid == 0 || myTid == 0 || fgTid == myTid {
		return SetForegroundWindow(hwnd)
	}

	r, _, _ := ProcAttachThreadInput.Call(myTid, fgTid, 1)
	if r == 0 {
		return SetForegroundWindow(hwnd)
	}
	defer ProcAttachThreadInput.Call(myTid, fgTid, 0)

	return SetForegroundWindow(hwnd)
}
