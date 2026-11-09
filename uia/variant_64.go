//go:build amd64 || arm64

package uia

import (
	"runtime"
	"syscall"
	"unsafe"
)

// variant matches the 64-bit Win32 VARIANT. The BRECORD member makes the
// struct 24 bytes. VT_I4 is stored at offset 8. The tail stays zero so a
// callee that copies the whole VARIANT does not read past this object.
// Windows passes a struct of this size by pointer; callCreatePropertyCondition
// passes that pointer.
type variant struct {
	VT        uint16
	Reserved1 uint16
	Reserved2 uint16
	Reserved3 uint16
	Val       int64
	pad       [8]byte
}

func callCreatePropertyCondition(a *iuiAutomation, propertyID uintptr, value variant) (*iuiAutomationCondition, uintptr) {
	var cond *iuiAutomationCondition
	hr, _, _ := syscall.SyscallN(
		a.lpVtbl.CreatePropertyCondition,
		uintptr(unsafe.Pointer(a)),
		propertyID,
		uintptr(unsafe.Pointer(&value)),
		uintptr(unsafe.Pointer(&cond)),
	)
	runtime.KeepAlive(&value)
	return cond, hr
}
