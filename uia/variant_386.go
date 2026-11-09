//go:build 386

package uia

import (
	"runtime"
	"syscall"
	"unsafe"
)

// variant matches the 32-bit Win32 VARIANT, which is 16 bytes.
// CreatePropertyCondition takes that struct by value on stdcall. A pointer
// would shift every later argument. The four words below are the in-memory
// little-endian layout, pushed as four stack arguments.
type variant struct {
	VT        uint16
	Reserved1 uint16
	Reserved2 uint16
	Reserved3 uint16
	Val       int64
}

func callCreatePropertyCondition(a *iuiAutomation, propertyID uintptr, value variant) (*iuiAutomationCondition, uintptr) {
	var cond *iuiAutomationCondition
	words := (*[4]uintptr)(unsafe.Pointer(&value))
	hr, _, _ := syscall.SyscallN(
		a.lpVtbl.CreatePropertyCondition,
		uintptr(unsafe.Pointer(a)),
		propertyID,
		words[0],
		words[1],
		words[2],
		words[3],
		uintptr(unsafe.Pointer(&cond)),
	)
	runtime.KeepAlive(&value)
	return cond, hr
}
