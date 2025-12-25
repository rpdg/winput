package mouse

import (
	"fmt"
	"syscall"

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

// makeLParam packs two 16-bit integers into a 32-bit uintptr.
// Note: It casts to int16 first to preserve sign behavior for negative coordinates.
func makeLParam(x, y int32) uintptr {
	lx := uint32(uint16(int16(x)))
	ly := uint32(uint16(int16(y)))
	return uintptr(lx | (ly << 16))
}

// makeWheelWParam packs delta (high word) and key states (low word).
func makeWheelWParam(delta int32, keyFlags uint16) uintptr {
	low := uint32(keyFlags)
	high := uint32(uint16(int16(delta)))
	return uintptr((high << 16) | low)
}

func post(hwnd uintptr, msg uint32, wparam uintptr, lparam uintptr) error {
	r, _, e := window.ProcPostMessageW.Call(hwnd, uintptr(msg), wparam, lparam)
	if r == 0 {
		if errno, ok := e.(syscall.Errno); ok && errno != 0 {
			return fmt.Errorf("PostMessageW failed: %w", errno)
		}
		return fmt.Errorf("PostMessageW failed")
	}
	return nil
}

func Move(hwnd uintptr, x, y int32) error {
	lparam := makeLParam(x, y)
	return post(hwnd, WM_MOUSEMOVE, 0, lparam)
}

func Click(hwnd uintptr, x, y int32) error {
	lparam := makeLParam(x, y)
	// We should propagate errors. If Move fails, Click should probably fail?
	// But Move (WM_MOUSEMOVE) failure might be ignorable? 
	// Better to be strict as per review.
	if err := post(hwnd, WM_MOUSEMOVE, 0, lparam); err != nil {
		return err
	}

	if err := post(hwnd, WM_LBUTTONDOWN, MK_LBUTTON, lparam); err != nil {
		return err
	}
	return post(hwnd, WM_LBUTTONUP, 0, lparam)
}

func ClickRight(hwnd uintptr, x, y int32) error {
	lparam := makeLParam(x, y)
	if err := post(hwnd, WM_MOUSEMOVE, 0, lparam); err != nil {
		return err
	}

	if err := post(hwnd, WM_RBUTTONDOWN, MK_RBUTTON, lparam); err != nil {
		return err
	}
	return post(hwnd, WM_RBUTTONUP, 0, lparam)
}

func ClickMiddle(hwnd uintptr, x, y int32) error {
	lparam := makeLParam(x, y)
	if err := post(hwnd, WM_MOUSEMOVE, 0, lparam); err != nil {
		return err
	}

	if err := post(hwnd, WM_MBUTTONDOWN, MK_MBUTTON, lparam); err != nil {
		return err
	}
	return post(hwnd, WM_MBUTTONUP, 0, lparam)
}

func DoubleClick(hwnd uintptr, x, y int32) error {
	lparam := makeLParam(x, y)
	if err := post(hwnd, WM_MOUSEMOVE, 0, lparam); err != nil {
		return err
	}

	if err := post(hwnd, WM_LBUTTONDBLCLK, MK_LBUTTON, lparam); err != nil {
		return err
	}
	return post(hwnd, WM_LBUTTONUP, 0, lparam)
}

// Scroll sends a vertical scroll message.
// delta is usually a multiple of 120 (WHEEL_DELTA). Positive = forward/up, Negative = backward/down.
// x, y are client coordinates where the scroll happens.
func Scroll(hwnd uintptr, x, y int32, delta int32) error {
	// WM_MOUSEWHEEL requires Screen Coordinates in LPARAM (low x, high y)
	sx, sy, err := window.ClientToScreen(hwnd, x, y)
	if err != nil {
		return err
	}

	wparam := makeWheelWParam(delta, 0)
	lparam := makeLParam(sx, sy)

	return post(hwnd, WM_MOUSEWHEEL, wparam, lparam)
}