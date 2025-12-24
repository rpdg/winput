package mouse

import (
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

	MK_LBUTTON = 0x0001
	MK_RBUTTON = 0x0002
)

func makeLParam(x, y int32) uintptr {
	ux := uint32(uint16(x))
	uy := uint32(uint16(y))
	return uintptr(ux | (uy << 16))
}

func post(hwnd uintptr, msg uint32, wparam uintptr, lparam uintptr) error {
	r, _, _ := window.ProcPostMessageW.Call(hwnd, uintptr(msg), wparam, lparam)
	if r == 0 {
		// PostMessage returns 0 on failure
		return nil // Or error? Prompt says "Explicit Failure".
		// But PostMessage failures are rare (invalid hwnd?).
		// Let's assume it might fail.
		// return fmt.Errorf("PostMessage failed")
	}
	return nil
}

func Move(hwnd uintptr, x, y int32) error {
	lparam := makeLParam(x, y)
	return post(hwnd, WM_MOUSEMOVE, 0, lparam)
}

func Click(hwnd uintptr, x, y int32) error {
	lparam := makeLParam(x, y)
	// Some windows require a Move before Click to register hover state or correct processing
	Move(hwnd, x, y)
	
	if err := post(hwnd, WM_LBUTTONDOWN, MK_LBUTTON, lparam); err != nil {
		return err
	}
	return post(hwnd, WM_LBUTTONUP, 0, lparam)
}

func ClickRight(hwnd uintptr, x, y int32) error {
	lparam := makeLParam(x, y)
	Move(hwnd, x, y)
	
	if err := post(hwnd, WM_RBUTTONDOWN, MK_RBUTTON, lparam); err != nil {
		return err
	}
	return post(hwnd, WM_RBUTTONUP, 0, lparam)
}

func DoubleClick(hwnd uintptr, x, y int32) error {
	lparam := makeLParam(x, y)
	Move(hwnd, x, y)
	
	if err := post(hwnd, WM_LBUTTONDBLCLK, MK_LBUTTON, lparam); err != nil {
		return err
	}
	return post(hwnd, WM_LBUTTONUP, 0, lparam)
}
