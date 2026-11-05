package keyboard

import (
	"errors"
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

// ErrUnsupportedKey means the Key encoding or the resulting virtual key is
// invalid. winput.ErrUnsupportedKey aliases this sentinel.
var ErrUnsupportedKey = errors.New("unsupported key or character")

var numpadVirtualKeys = map[uint8]uintptr{
	0x52: 0x60, // VK_NUMPAD0
	0x4F: 0x61, // VK_NUMPAD1
	0x50: 0x62, // VK_NUMPAD2
	0x51: 0x63, // VK_NUMPAD3
	0x4B: 0x64, // VK_NUMPAD4
	0x4C: 0x65, // VK_NUMPAD5
	0x4D: 0x66, // VK_NUMPAD6
	0x47: 0x67, // VK_NUMPAD7
	0x48: 0x68, // VK_NUMPAD8
	0x49: 0x69, // VK_NUMPAD9
	0x53: 0x6E, // VK_DECIMAL
}

// MapScanCodeToVK converts a key to a virtual-key code using type 1
// (MAPVK_VSC_TO_VK). It returns 0 when the key is invalid.
func MapScanCodeToVK(sc Key) uintptr {
	vk, _ := VirtualKey(sc)
	return vk
}

// VirtualKey returns the virtual key for the PostMessage wParam.
// Numpad 0-9 and decimal are explicit because MapVirtualKeyW cannot
// distinguish them from the extended navigation keys. Everything else uses
// MapVirtualKeyW type 1.
func VirtualKey(key Key) (uintptr, bool) {
	scan, extended, ok := key.Decode()
	if !ok {
		return 0, false
	}
	if !extended {
		if vk, ok := numpadVirtualKeys[scan]; ok {
			return vk, true
		}
	}
	r, _, _ := window.ProcMapVirtualKeyW.Call(uintptr(key), MAPVK_VSC_TO_VK)
	if r == 0 {
		return 0, false
	}
	return r, true
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

func makeKeyLParam(scan uint8, extended bool, isUp bool) uintptr {
	var lparam uintptr
	// Repeat count = 1
	lparam |= 1
	// Scan code (bits 16-23)
	lparam |= uintptr(scan) << 16

	// Extended key flag (bit 24)
	if extended {
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
	scan, extended, ok := key.Decode()
	if !ok {
		return ErrUnsupportedKey
	}
	vk, ok := VirtualKey(key)
	if !ok {
		return ErrUnsupportedKey
	}
	return post(hwnd, WM_KEYDOWN, vk, makeKeyLParam(scan, extended, false))
}

// KeyUp simulates a key up event for the specified window using PostMessage.
func KeyUp(hwnd uintptr, key Key) error {
	scan, extended, ok := key.Decode()
	if !ok {
		return ErrUnsupportedKey
	}
	vk, ok := VirtualKey(key)
	if !ok {
		return ErrUnsupportedKey
	}
	return post(hwnd, WM_KEYUP, vk, makeKeyLParam(scan, extended, true))
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
