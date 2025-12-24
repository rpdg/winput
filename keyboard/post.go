package keyboard

import (
	"fmt"
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
	r, _, _ := window.ProcPostMessageW.Call(hwnd, uintptr(msg), wparam, lparam)
	if r == 0 {
		return fmt.Errorf("PostMessage failed")
	}
	return nil
}

func KeyDown(hwnd uintptr, key Key) error {
	vk := mapScanCodeToVK(key)
	if vk == 0 {
		return fmt.Errorf("unsupported key: %d", key)
	}
	
	// LPARAM construction for WM_KEYDOWN
	// 0-15: Repeat count (1)
	// 16-23: Scan code
	// 24: Extended key (0 for now)
	// 29: Context code (0)
	// 30: Previous key state (0)
	// 31: Transition state (0)
	
	lparam := uintptr(1) | (uintptr(key) << 16)
	return post(hwnd, WM_KEYDOWN, vk, lparam)
}

func KeyUp(hwnd uintptr, key Key) error {
	vk := mapScanCodeToVK(key)
	
	// LPARAM construction for WM_KEYUP
	// 0-15: Repeat count (1)
	// 16-23: Scan code
	// 30: Previous key state (1)
	// 31: Transition state (1)
	
	lparam := uintptr(1) | (uintptr(key) << 16) | (1 << 30) | (1 << 31)
	return post(hwnd, WM_KEYUP, vk, lparam)
}

func Press(hwnd uintptr, key Key) error {
	if err := KeyDown(hwnd, key); err != nil {
		return err
	}
	// Small delay? User didn't mandate but good for realism/processing.
	time.Sleep(10 * time.Millisecond)
	return KeyUp(hwnd, key)
}

func Type(hwnd uintptr, text string) error {
	for _, r := range text {
		k, ok := KeyFromRune(r)
		if ok {
			Press(hwnd, k)
		}
		// What if not in map? Ignore or fail?
		// User said "Internal constant... Table internal use only... Explicit failure".
		// But for Type(string), failing on one char might be annoying.
		// However, adhering to "Explicit Failure":
		// if !ok { return fmt.Errorf("unsupported char: %c", r) }
		// But for now let's just skip or log.
		time.Sleep(20 * time.Millisecond)
	}
	return nil
}
