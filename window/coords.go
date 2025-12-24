package window

import (
	"fmt"
	"unsafe"
)

type POINT struct {
	X, Y int32
}

type RECT struct {
	Left, Top, Right, Bottom int32
}

func IsIconic(hwnd uintptr) bool {
	r, _, _ := ProcIsIconic.Call(hwnd)
	return r != 0
}

func IsValid(hwnd uintptr) bool {
	r, _, _ := ProcIsWindow.Call(hwnd)
	return r != 0
}

func IsVisible(hwnd uintptr) bool {
	r, _, _ := ProcIsWindowVisible.Call(hwnd)
	return r != 0
}

func GetClientRect(hwnd uintptr) (width, height int32, err error) {
	if IsIconic(hwnd) {
		return 0, 0, fmt.Errorf("window is minimized")
	}
	var rc RECT
	r, _, _ := ProcGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	if r == 0 {
		return 0, 0, fmt.Errorf("GetClientRect failed")
	}
	return rc.Right - rc.Left, rc.Bottom - rc.Top, nil
}

func ScreenToClient(hwnd uintptr, x, y int32) (cx, cy int32, err error) {
	if IsIconic(hwnd) {
		return 0, 0, fmt.Errorf("window is minimized")
	}
	pt := POINT{X: x, Y: y}
	r, _, _ := ProcScreenToClient.Call(hwnd, uintptr(unsafe.Pointer(&pt)))
	if r == 0 {
		return 0, 0, fmt.Errorf("ScreenToClient failed")
	}
	return pt.X, pt.Y, nil
}

func ClientToScreen(hwnd uintptr, x, y int32) (sx, sy int32, err error) {
	if IsIconic(hwnd) {
		return 0, 0, fmt.Errorf("window is minimized")
	}
	pt := POINT{X: x, Y: y}
	r, _, _ := ProcClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&pt)))
	if r == 0 {
		return 0, 0, fmt.Errorf("ClientToScreen failed")
	}
	return pt.X, pt.Y, nil
}

func GetCursorPos() (x, y int32, err error) {
	var pt POINT
	r, _, _ := ProcGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	if r == 0 {
		return 0, 0, fmt.Errorf("GetCursorPos failed")
	}
	return pt.X, pt.Y, nil
}
