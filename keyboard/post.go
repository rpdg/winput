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

	MAPVK_VSC_TO_VK = 1
)

func mapScanCodeToVK(sc Key) uintptr {
	r, _, _ := window.ProcMapVirtualKeyW.Call(uintptr(sc), MAPVK_VSC_TO_VK)
	return r
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

func KeyDown(hwnd uintptr, key Key) error {
	vk := mapScanCodeToVK(key)
	if vk == 0 {
		return fmt.Errorf("unsupported key: %d", key)
	}

	lparam := uintptr(1) | (uintptr(key) << 16)
	return post(hwnd, WM_KEYDOWN, vk, lparam)
}

func KeyUp(hwnd uintptr, key Key) error {
	vk := mapScanCodeToVK(key)
	if vk == 0 {
		return fmt.Errorf("unsupported key: %d", key)
	}

	lparam := uintptr(1) | (uintptr(key) << 16) | (1 << 30) | (1 << 31)
	return post(hwnd, WM_KEYUP, vk, lparam)
}

func Press(hwnd uintptr, key Key) error {
	if err := KeyDown(hwnd, key); err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond)
	return KeyUp(hwnd, key)
}

func Type(hwnd uintptr, text string) error {
	for _, r := range text {
		k, shifted, ok := LookupKey(r)
		if ok {
			if shifted {
				if err := KeyDown(hwnd, KeyShift); err != nil {
					return err
				}
				if err := Press(hwnd, k); err != nil {
					return err
				}
				if err := KeyUp(hwnd, KeyShift); err != nil {
					return err
				}
			} else {
				if err := Press(hwnd, k); err != nil {
					return err
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	return nil
}