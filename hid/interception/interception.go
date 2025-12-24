package interception

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	dllHandle syscall.Handle

	procCreateContext  uintptr
	procDestroyContext uintptr
	procIsMouse        uintptr
	procIsKeyboard     uintptr
	procSend           uintptr
)

// Default library name
var libraryPath = "interception.dll"

// SetLibraryPath sets the path for LoadLibrary.
func SetLibraryPath(path string) {
	libraryPath = path
}

var (
	ErrLibraryNotFound = fmt.Errorf("interception library not found")
)

// Load loads the interception.dll and resolves function addresses.
func Load() error {
	if dllHandle != 0 {
		return nil
	}

	h, err := syscall.LoadLibrary(libraryPath)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrLibraryNotFound, err)
	}
	dllHandle = h

	procCreateContext = getProc(h, "interception_create_context")
	procDestroyContext = getProc(h, "interception_destroy_context")
	procIsMouse = getProc(h, "interception_is_mouse")
	procIsKeyboard = getProc(h, "interception_is_keyboard")
	procSend = getProc(h, "interception_send")

	// Check essential functions
	if procCreateContext == 0 || procSend == 0 {
		syscall.FreeLibrary(h)
		dllHandle = 0
		return fmt.Errorf("library loaded but symbols missing")
	}

	return nil
}

func getProc(h syscall.Handle, name string) uintptr {
	addr, _ := syscall.GetProcAddress(h, name)
	return addr
}

// Types

type Context uintptr
type Device int

// Go-friendly structs
type MouseStroke struct {
	State       uint16
	Flags       uint16
	Rolling     int16
	X           int32
	Y           int32
	Information uint32
}

type KeyStroke struct {
	Code        uint16
	State       uint16
	Information uint32
}

// Constants for Mouse (Manually copied from header)
const (
	MouseStateLeftDown   = 0x001
	MouseStateLeftUp     = 0x002
	MouseStateRightDown  = 0x004
	MouseStateRightUp    = 0x008
	MouseStateMiddleDown = 0x010
	MouseStateMiddleUp   = 0x020
	MouseStateWheel      = 0x400

	MouseFlagMoveRelative = 0x000
	MouseFlagMoveAbsolute = 0x001
)

// Constants for Keyboard
const (
	KeyStateDown = 0x00
	KeyStateUp   = 0x01
	KeyStateE0   = 0x02
	KeyStateE1   = 0x04
)

// Functions Wrappers

func CreateContext() Context {
	if procCreateContext == 0 {
		return 0
	}
	r, _, _ := syscall.Syscall(procCreateContext, 0, 0, 0, 0)
	return Context(r)
}

func DestroyContext(ctx Context) {
	if procDestroyContext == 0 {
		return
	}
	syscall.Syscall(procDestroyContext, 1, uintptr(ctx), 0, 0)
}

func IsMouse(dev Device) bool {
	if procIsMouse == 0 {
		return false
	}
	r, _, _ := syscall.Syscall(procIsMouse, 1, uintptr(dev), 0, 0)
	return r != 0
}

func IsKeyboard(dev Device) bool {
	if procIsKeyboard == 0 {
		return false
	}
	r, _, _ := syscall.Syscall(procIsKeyboard, 1, uintptr(dev), 0, 0)
	return r != 0
}

func SendMouse(ctx Context, dev Device, s *MouseStroke) {
	if procSend == 0 {
		return
	}
	// InterceptionStroke is [sizeof(MouseStroke)]byte.
	// We can pass the pointer to our struct directly as it matches the layout.
	// Signature: int interception_send(Context, Device, Stroke*, nStroke)
	syscall.Syscall6(procSend, 4, uintptr(ctx), uintptr(dev), uintptr(unsafe.Pointer(s)), 1, 0, 0)
}

func SendKey(ctx Context, dev Device, s *KeyStroke) {
	if procSend == 0 {
		return
	}
	// Note: KeyStroke is smaller than MouseStroke. 
	// Interception expects a pointer to a buffer of size INTERCEPTION_STROKE (which is max of mouse/key).
	// But interception_send implementation likely only reads relevant fields based on device type.
	// However, to be safe and match the C logic (casting to array of char), we should ensure memory safety.
	// Go structs are memory compatible here.
	syscall.Syscall6(procSend, 4, uintptr(ctx), uintptr(dev), uintptr(unsafe.Pointer(s)), 1, 0, 0)
}