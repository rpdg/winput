package keyboard

import (
	"fmt"
	"syscall"
	"time"

	"github.com/rpdg/winput/window"
)

const (
	WM_KEYDOWN = 0x0100
	WM_KEYUP   = 0x0101
	WM_CHAR    = 0x0102

	MAPVK_VSC_TO_VK = 1
)

// MapScanCodeToVK converts a hardware scan code to a virtual-key code.
func MapScanCodeToVK(sc Key) uintptr {
	r, _, _ := window.ProcMapVirtualKeyW.Call(uintptr(sc), MAPVK_VSC_TO_VK)
	return r
}

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

func makeKeyLParam(sc Key, isUp bool) uintptr {
	var lparam uintptr
	// Repeat count = 1
	lparam |= 1
	// Scan code (bits 16-23)
	lparam |= (uintptr(sc) & 0xFF) << 16

	// Extended key flag (bit 24)
	if isExtended(sc) {
		lparam |= 1 << 24
	}

	if isUp {
		// Previous state (bit 30) + Transition state (bit 31)
		lparam |= 1<<30 | 1<<31
	}
	return lparam
}

// KeyDown simulates a key down event for the specified window using PostMessage.
func KeyDown(hwnd uintptr, key Key) error {
	vk := MapScanCodeToVK(key)
	if vk == 0 {
		return fmt.Errorf("unsupported key: %d", key)
	}
	lparam := makeKeyLParam(key, false)
	return post(hwnd, WM_KEYDOWN, vk, lparam)
}

// KeyUp simulates a key up event for the specified window using PostMessage.
func KeyUp(hwnd uintptr, key Key) error {
	vk := MapScanCodeToVK(key)
	if vk == 0 {
		return fmt.Errorf("unsupported key: %d", key)
	}
	lparam := makeKeyLParam(key, true)
	return post(hwnd, WM_KEYUP, vk, lparam)
}

// Press simulates a key press (down then up) for the specified window using PostMessage.
func Press(hwnd uintptr, key Key) error {
	if err := KeyDown(hwnd, key); err != nil {
		return err
	}
	time.Sleep(30 * time.Millisecond)
	return KeyUp(hwnd, key)
}

// Type sends text to the specified window using WM_CHAR messages.
// This is reliable for background input but does not support non-character keys.
func Type(hwnd uintptr, text string) error {
	for _, r := range text {
		if r > 0xFFFF {
			r -= 0x10000
			high := 0xD800 + (r >> 10)
			low := 0xDC00 + (r & 0x3FF)
			if err := post(hwnd, WM_CHAR, uintptr(high), 1); err != nil {
				return err
			}
			if err := post(hwnd, WM_CHAR, uintptr(low), 1); err != nil {
				return err
			}
		} else {
			if err := post(hwnd, WM_CHAR, uintptr(r), 1); err != nil {
				return err
			}
		}
		time.Sleep(30 * time.Millisecond)
	}
	return nil
}
