package interception

import (
	"encoding/binary"
	"fmt"
	"runtime"
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

var (
	ErrLibraryNotFound = fmt.Errorf("interception library not found")
	ErrSendFailed      = fmt.Errorf("interception_send failed")
)

// Default library name
var libraryPath = "interception.dll"

// SetLibraryPath sets the path for LoadLibrary.
func SetLibraryPath(path string) {
	libraryPath = path
}

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

// Unload frees the loaded DLL.
func Unload() {
	if dllHandle != 0 {
		syscall.FreeLibrary(dllHandle)
		dllHandle = 0
		procCreateContext = 0
		procDestroyContext = 0
		procIsMouse = 0
		procIsKeyboard = 0
		procSend = 0
	}
}

func getProc(h syscall.Handle, name string) uintptr {
	addr, _ := syscall.GetProcAddress(h, name)
	return addr
}

// Types

type (
	Context uintptr
	Device  int
)

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

// Ensure Memory Safety:
// We use a fixed buffer size that accommodates the C struct padding.
// C MouseStroke: 2+2+2 + 2(pad) + 4+4+4 = 20 bytes.
// But we allocate slightly more to be safe against compiler variations.
var strokeSize = 24

// Constants for Mouse
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

func SendMouse(ctx Context, dev Device, s *MouseStroke) error {
	if procSend == 0 {
		return fmt.Errorf("interception_send missing")
	}

	buf := make([]byte, strokeSize)
	// Offset 0: State (2 bytes)
	binary.LittleEndian.PutUint16(buf[0:2], s.State)
	// Offset 2: Flags (2 bytes)
	binary.LittleEndian.PutUint16(buf[2:4], s.Flags)
	// Offset 4: Rolling (2 bytes)
	binary.LittleEndian.PutUint16(buf[4:6], uint16(s.Rolling))

	// Offset 6: Padding (2 bytes) - Implicitly 0

	// Offset 8: X (4 bytes) - Correctly aligned
	binary.LittleEndian.PutUint32(buf[8:12], uint32(s.X))
	// Offset 12: Y (4 bytes)
	binary.LittleEndian.PutUint32(buf[12:16], uint32(s.Y))
	// Offset 16: Information (4 bytes)
	binary.LittleEndian.PutUint32(buf[16:20], s.Information)

	return send(ctx, dev, buf[:20]) // Send exactly 20 bytes as expected by C struct
}

func SendKey(ctx Context, dev Device, s *KeyStroke) error {
	if procSend == 0 {
		return fmt.Errorf("interception_send missing")
	}

	// KeyStroke is naturally aligned: 2+2+4 = 8 bytes.
	// No padding needed.
	buf := make([]byte, strokeSize)
	binary.LittleEndian.PutUint16(buf[0:2], s.Code)
	binary.LittleEndian.PutUint16(buf[2:4], s.State)
	binary.LittleEndian.PutUint32(buf[4:8], s.Information)

	return send(ctx, dev, buf[:8]) // Send 8 bytes
}

func send(ctx Context, dev Device, buf []byte) error {
	if len(buf) == 0 {
		return fmt.Errorf("empty buffer")
	}
	// Pass pointer to first element in single expression to satisfy unsafe rules.
	r, _, e := syscall.Syscall6(procSend, 4, uintptr(ctx), uintptr(dev), uintptr(unsafe.Pointer(&buf[0])), 1, 0, 0)
	if r == 0 {
		if e != 0 {
			return e
		}
		return ErrSendFailed
	}
	// Ensure buf is kept alive until syscall returns
	runtime.KeepAlive(buf)
	return nil
}
