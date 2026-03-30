package window

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

const (
	WM_GETTEXT       = 0x000D
	WM_GETTEXTLENGTH = 0x000E

	SMTO_ABORTIFHUNG = 0x0002
)

var ErrReadTextFailed = errors.New("failed to read window text")

func sendMessageTimeout(hwnd uintptr, msg uint32, wparam uintptr, lparam uintptr, timeoutMs uint32) (uintptr, error) {
	var result uintptr
	r, _, e := ProcSendMessageTimeoutW.Call(
		hwnd,
		uintptr(msg),
		wparam,
		lparam,
		SMTO_ABORTIFHUNG,
		uintptr(timeoutMs),
		uintptr(unsafe.Pointer(&result)),
	)
	if r == 0 {
		if errno, ok := e.(syscall.Errno); ok && errno != 0 {
			return 0, fmt.Errorf("%w: %v", ErrReadTextFailed, errno)
		}
		return 0, ErrReadTextFailed
	}
	return result, nil
}

func getWindowText(hwnd uintptr, length int) (string, error) {
	buf := make([]uint16, length+1)
	r, _, e := ProcGetWindowTextW.Call(
		hwnd,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	if r == 0 {
		if errno, ok := e.(syscall.Errno); ok && errno != 0 {
			return "", fmt.Errorf("%w: %v", ErrReadTextFailed, errno)
		}
	}
	return syscall.UTF16ToString(buf), nil
}

// GetText returns the current text for a window/control handle.
// It prefers WM_GETTEXT to support standard text controls, then falls back to GetWindowTextW.
func GetText(hwnd uintptr) (string, error) {
	if !IsValid(hwnd) {
		return "", fmt.Errorf("%w: invalid handle", ErrReadTextFailed)
	}

	length, err := sendMessageTimeout(hwnd, WM_GETTEXTLENGTH, 0, 0, 200)
	if err == nil {
		buf := make([]uint16, int(length)+1)
		if _, err := sendMessageTimeout(
			hwnd,
			WM_GETTEXT,
			uintptr(len(buf)),
			uintptr(unsafe.Pointer(&buf[0])),
			200,
		); err == nil {
			return syscall.UTF16ToString(buf), nil
		}
	}

	n, _, e := ProcGetWindowTextLengthW.Call(hwnd)
	if n == 0 {
		if errno, ok := e.(syscall.Errno); ok && errno != 0 {
			return "", fmt.Errorf("%w: %v", ErrReadTextFailed, errno)
		}
	}

	return getWindowText(hwnd, int(n))
}
