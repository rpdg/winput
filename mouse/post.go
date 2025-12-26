package mouse

import (
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
)

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
// It clips coordinates to 16-bit signed integer range to prevent overflow behavior.
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

func Move(hwnd uintptr, x, y int32) error {
	return post(hwnd, WM_MOUSEMOVE, 0, makeLParam(x, y))
}

func Click(hwnd uintptr, x, y int32) error {
	if err := Move(hwnd, x, y); err != nil {
		return err
	}
	lparam := makeLParam(x, y)
	if err := post(hwnd, WM_LBUTTONDOWN, MK_LBUTTON, lparam); err != nil {
		return err
	}
	// Small delay to simulate click duration, though PostMessage is async
	time.Sleep(10 * time.Millisecond)
	if err := post(hwnd, WM_LBUTTONUP, 0, lparam); err != nil {
		return err
	}
	return nil
}

func ClickRight(hwnd uintptr, x, y int32) error {
	if err := Move(hwnd, x, y); err != nil {
		return err
	}
	lparam := makeLParam(x, y)
	if err := post(hwnd, WM_RBUTTONDOWN, MK_RBUTTON, lparam); err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond)
	if err := post(hwnd, WM_RBUTTONUP, 0, lparam); err != nil {
		return err
	}
	return nil
}

func ClickMiddle(hwnd uintptr, x, y int32) error {
	if err := Move(hwnd, x, y); err != nil {
		return err
	}
	lparam := makeLParam(x, y)
	if err := post(hwnd, WM_MBUTTONDOWN, MK_MBUTTON, lparam); err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond)
	if err := post(hwnd, WM_MBUTTONUP, 0, lparam); err != nil {
		return err
	}
	return nil
}

func DoubleClick(hwnd uintptr, x, y int32) error {
	// Standard double click sequence: Down, Up, DoubleClick, Up
	if err := Click(hwnd, x, y); err != nil {
		return err
	}
	time.Sleep(50 * time.Millisecond) // Typical double click interval
	lparam := makeLParam(x, y)
	if err := post(hwnd, WM_LBUTTONDBLCLK, MK_LBUTTON, lparam); err != nil {
		return err
	}
	if err := post(hwnd, WM_LBUTTONUP, 0, lparam); err != nil {
		return err
	}
	return nil
}

// Scroll sends a vertical scroll message.
// delta is usually a multiple of 120 (WHEEL_DELTA). Positive = forward/up, Negative = backward/down.
// x, y are client coordinates where the scroll happens.
func Scroll(hwnd uintptr, x, y int32, delta int32) error {
	// WM_MOUSEWHEEL expects screen coordinates in LPARAM!
	// Low-order word: x, High-order word: y
	sx, sy, err := window.ClientToScreen(hwnd, x, y)
	if err != nil {
		return err
	}

	// Prepare WPARAM: High-order word is delta, Low-order word is keys (0)
	wparam := uintptr(uint16(0)) | (uintptr(uint16(delta)) << 16)
	lparam := makeLParam(sx, sy)

	return post(hwnd, WM_MOUSEWHEEL, wparam, lparam)
}
