package window

import (
	"fmt"
	"unsafe"
)

// POINT represents a point in 2D space (x, y).
// It corresponds to the Win32 POINT structure.
type POINT struct {
	X, Y int32
}

// RECT represents a rectangle in 2D space.
// It corresponds to the Win32 RECT structure.
type RECT struct {
	Left, Top, Right, Bottom int32
}

// IsIconic checks if the specified window is minimized (iconic).
func IsIconic(hwnd uintptr) bool {
	r, _, _ := ProcIsIconic.Call(hwnd)
	return r != 0
}

// IsValid checks if the specified window handle identifies an existing window.
func IsValid(hwnd uintptr) bool {
	r, _, _ := ProcIsWindow.Call(hwnd)
	return r != 0
}

// IsVisible checks if the specified window has the WS_VISIBLE style.
func IsVisible(hwnd uintptr) bool {
	r, _, _ := ProcIsWindowVisible.Call(hwnd)
	return r != 0
}

// GetClientRect retrieves the coordinates of a window's client area.
// The client coordinates specify the upper-left and lower-right corners of the
// client area. Because client coordinates are relative to the upper-left corner
// of a window's client area, the coordinates of the upper-left corner are (0,0).
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

// ScreenToClient converts the screen coordinates of a specified point on the screen
// to client-area coordinates.
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

// ClientToScreen converts the client-area coordinates of a specified point to
// screen coordinates.
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

// GetCursorPos retrieves the cursor's position, in screen coordinates.
// The coordinates are relative to the primary monitor (0,0).
// Returns negative values if the cursor is on a monitor to the left or above the primary monitor.
func GetCursorPos() (x, y int32, err error) {
	var pt POINT
	r, _, _ := ProcGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	if r == 0 {
		return 0, 0, fmt.Errorf("GetCursorPos failed")
	}
	return pt.X, pt.Y, nil
}

// SetCursorPos moves the cursor to the specified screen coordinates.
func SetCursorPos(x, y int32) error {
	r, _, _ := ProcSetCursorPos.Call(uintptr(x), uintptr(y))
	if r == 0 {
		return fmt.Errorf("SetCursorPos failed")
	}
	return nil
}
