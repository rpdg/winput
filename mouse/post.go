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

var ErrInvalidScrollDelta = errors.New("scroll delta out of int16 range")

// ErrInvalidMouseCoordinate means a client or screen coordinate does not fit
// in the signed 16-bit halves of a mouse-message lParam.
var ErrInvalidMouseCoordinate = errors.New("mouse coordinate out of int16 range")

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
// Coordinates that do not fit in int16 are rejected. They are not clipped:
// a clipped message would land at the wrong point and still look successful.
func makeLParam(x, y int32) (uintptr, error) {
	if x > 32767 || x < -32768 || y > 32767 || y < -32768 {
		return 0, ErrInvalidMouseCoordinate
	}
	return uintptr(uint16(int16(x))) | (uintptr(uint16(int16(y))) << 16), nil
}

func wheelDelta(delta int32) (int16, error) {
	if delta < -32768 || delta > 32767 {
		return 0, ErrInvalidScrollDelta
	}
	return int16(delta), nil
}

// buttonMessage is one message in a button sequence.
// up marks a button-up step. Only those steps are retried.
type buttonMessage struct {
	msg    uint32
	wparam uintptr
	up     bool
}

// doubleClickSequence is the four-step window double-click sequence.
func doubleClickSequence() [4]buttonMessage {
	return [4]buttonMessage{
		{WM_LBUTTONDOWN, MK_LBUTTON, false},
		{WM_LBUTTONUP, 0, true},
		{WM_LBUTTONDBLCLK, MK_LBUTTON, false},
		{WM_LBUTTONUP, 0, true},
	}
}

func postButtonUp(hwnd uintptr, msg uint32, lparam uintptr) error {
	if err := post(hwnd, msg, 0, lparam); err != nil {
		// The button is logically down until this message arrives; retry once.
		if err2 := post(hwnd, msg, 0, lparam); err2 != nil {
			return err
		}
	}
	return nil
}

// Move simulates a mouse move event to the specified client coordinates using PostMessage.
func Move(hwnd uintptr, x, y int32) error {
	lparam, err := makeLParam(x, y)
	if err != nil {
		return err
	}
	return post(hwnd, WM_MOUSEMOVE, 0, lparam)
}

// Click simulates a left mouse button click at the specified client coordinates.
func Click(hwnd uintptr, x, y int32) error {
	lparam, err := makeLParam(x, y)
	if err != nil {
		return err
	}
	if err := post(hwnd, WM_LBUTTONDOWN, MK_LBUTTON, lparam); err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond)
	return postButtonUp(hwnd, WM_LBUTTONUP, lparam)
}

// ClickRight simulates a right mouse button click at the specified client coordinates.
func ClickRight(hwnd uintptr, x, y int32) error {
	lparam, err := makeLParam(x, y)
	if err != nil {
		return err
	}
	if err := post(hwnd, WM_RBUTTONDOWN, MK_RBUTTON, lparam); err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond)
	return postButtonUp(hwnd, WM_RBUTTONUP, lparam)
}

// ClickMiddle simulates a middle mouse button click at the specified client coordinates.
func ClickMiddle(hwnd uintptr, x, y int32) error {
	lparam, err := makeLParam(x, y)
	if err != nil {
		return err
	}
	if err := post(hwnd, WM_MBUTTONDOWN, MK_MBUTTON, lparam); err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond)
	return postButtonUp(hwnd, WM_MBUTTONUP, lparam)
}

// DoubleClick simulates a left mouse button double-click at the specified client coordinates.
func DoubleClick(hwnd uintptr, x, y int32) error {
	lparam, err := makeLParam(x, y)
	if err != nil {
		return err
	}
	for _, step := range doubleClickSequence() {
		err := post(hwnd, step.msg, step.wparam, lparam)
		if err == nil {
			continue
		}
		// Only a button-up step is retried, and only that step.
		if step.up {
			if err2 := post(hwnd, step.msg, step.wparam, lparam); err2 == nil {
				continue
			}
		}
		return err
	}
	return nil
}

// Scroll simulates a vertical mouse wheel scroll at the specified coordinates.
// delta must fit in an int16. A zero delta is a no-op.
func Scroll(hwnd uintptr, x, y int32, delta int32) error {
	if delta == 0 {
		return nil
	}
	d, err := wheelDelta(delta)
	if err != nil {
		return err
	}

	sx, sy, err := window.ClientToScreen(hwnd, x, y)
	if err != nil {
		return err
	}

	// High-order word is the signed delta.
	wparam := uintptr(uint16(d)) << 16
	lparam, err := makeLParam(sx, sy)
	if err != nil {
		return err
	}

	return post(hwnd, WM_MOUSEWHEEL, wparam, lparam)
}
