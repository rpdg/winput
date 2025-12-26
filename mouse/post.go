package mouse

import (
	"errors"
	"fmt"
	"syscall"
	"time"

	"github.com/rpdg/winput/window"
)

const (
	WM_MOUSEMOVE     = 0x0200
	WM_LBUTTONDOWN   = 0x0201
	WM_LBUTTONUP     = 0x0202
	WM_LBUTTONDBLCLK = 0x0203
	WM_RBUTTONDOWN   = 0x0204
	WM_RBUTTONUP     = 0x0205
	WM_RBUTTONDBLCLK = 0x0206
	WM_MBUTTONDOWN   = 0x0207
	WM_MBUTTONUP     = 0x0208
	WM_MBUTTONDBLCLK = 0x0209
	WM_MOUSEWHEEL    = 0x020A

	MK_LBUTTON = 0x0001
	MK_RBUTTON = 0x0002
	MK_MBUTTON = 0x0010

	WHEEL_DELTA = 120
)

var ErrInvalidScrollDelta = errors.New("scroll delta must be a multiple of WHEEL_DELTA (120)")

// Helper to check for errors and wrap errno
func post(hwnd uintptr, msg uint32, wparam uintptr, lparam uintptr) error {
	r, _, e := window.ProcPostMessageW.Call(hwnd, uintptr(msg), wparam, lparam)
	if r == 0 {
		if errno, ok := e.(syscall.Errno); ok && errno != 0 {
			return fmt.Errorf("%w: %v", window.ErrPostMessageFailed, errno)
		}
		return window.ErrPostMessageFailed
	}
	return nil
}

// makeLParam constructs the LPARAM for mouse messages.
func makeLParam(x, y int32) uintptr {
	lx := clipToInt16(x)
	ly := clipToInt16(y)
	return uintptr(uint16(lx)) | (uintptr(uint16(ly)) << 16)
}

func clipToInt16(v int32) int16 {
	if v > 32767 {
		return 32767
	}
	if v < -32768 {
		return -32768
	}
	return int16(v)
}

// Move simulates a mouse move event to the specified client coordinates using PostMessage.
func Move(hwnd uintptr, x, y int32) error {
	return post(hwnd, WM_MOUSEMOVE, 0, makeLParam(x, y))
}

// Click simulates a left mouse button click at the specified client coordinates.
func Click(hwnd uintptr, x, y int32) error {
	lparam := makeLParam(x, y)
	if err := post(hwnd, WM_LBUTTONDOWN, MK_LBUTTON, lparam); err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond)
	return post(hwnd, WM_LBUTTONUP, 0, lparam)
}

// ClickRight simulates a right mouse button click at the specified client coordinates.
func ClickRight(hwnd uintptr, x, y int32) error {
	lparam := makeLParam(x, y)
	if err := post(hwnd, WM_RBUTTONDOWN, MK_RBUTTON, lparam); err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond)
	return post(hwnd, WM_RBUTTONUP, 0, lparam)
}

// ClickMiddle simulates a middle mouse button click at the specified client coordinates.
func ClickMiddle(hwnd uintptr, x, y int32) error {
	lparam := makeLParam(x, y)
	if err := post(hwnd, WM_MBUTTONDOWN, MK_MBUTTON, lparam); err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond)
	return post(hwnd, WM_MBUTTONUP, 0, lparam)
}

// DoubleClick simulates a left mouse button double-click at the specified client coordinates.
func DoubleClick(hwnd uintptr, x, y int32) error {
	lparam := makeLParam(x, y)
	if err := post(hwnd, WM_LBUTTONDBLCLK, MK_LBUTTON, lparam); err != nil {
		return err
	}
	return post(hwnd, WM_LBUTTONUP, 0, lparam)
}

// Scroll simulates a vertical mouse wheel scroll at the specified coordinates.
// delta must be a multiple of WHEEL_DELTA (120).
func Scroll(hwnd uintptr, x, y int32, delta int32) error {
	if delta%WHEEL_DELTA != 0 {
		return ErrInvalidScrollDelta
	}

	sx, sy, err := window.ClientToScreen(hwnd, x, y)
	if err != nil {
		return err
	}

	// High-order word is signed delta
	wparam := uintptr(uint16(0)) | (uintptr(int16(delta)) << 16)
	lparam := makeLParam(sx, sy)

	return post(hwnd, WM_MOUSEWHEEL, wparam, lparam)
}