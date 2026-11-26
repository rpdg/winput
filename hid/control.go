// Package hid drives the Interception backend.
//
// The exported functions are not safe for concurrent use. Callers must
// serialize them. winput does this with its inputMutex.
package hid

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/rpdg/winput/hid/interception"
	"github.com/rpdg/winput/internal/motion"
	"github.com/rpdg/winput/keyboard"
	"github.com/rpdg/winput/screen"
	"github.com/rpdg/winput/window"
)

var (
	ErrDriverNotInstalled    = errors.New("interception driver not installed or accessible")
	ErrMoveNotConverged      = errors.New("HID move did not converge")
	ErrOutsideVirtualDesktop = errors.New("target outside virtual desktop")
	ErrInvalidScrollDelta    = errors.New("scroll delta out of int16 range")

	// ErrLibraryNotFound is the load failure for a missing interception library.
	// ErrSymbolsMissing is the load failure when the library is present but
	// required exports are absent. ErrDevicesUnavailable is returned after a
	// context was created and then destroyed because neither a mouse nor a
	// keyboard device was found.
	ErrLibraryNotFound    = interception.ErrLibraryNotFound
	ErrSymbolsMissing     = interception.ErrSymbolsMissing
	ErrDevicesUnavailable = errors.New("no interception devices found")
)

// SetLibraryPath sets the custom path for the interception.dll library.
func SetLibraryPath(path string) {
	interception.SetLibraryPath(path)
}

const (
	MaxInterceptionDevices = 20
)

var (
	ctx         interception.Context
	mouseDev    interception.Device
	keyboardDev interception.Device
	initialized bool
	// initMutex protects the initialized state and the context/device handles.
	// RLock is held during ANY input operation to prevent Close() from destroying
	// the context mid-operation.
	initMutex sync.RWMutex
)

type deviceScanner struct {
	isMouse       func(interception.Device) bool
	isKeyboard    func(interception.Device) bool
	getHardwareID func(interception.Context, interception.Device) ([]string, error)
}

var (
	interceptionLoad           = interception.Load
	interceptionUnload         = interception.Unload
	interceptionCreateContext  = interception.CreateContext
	interceptionDestroyContext = interception.DestroyContext
	defaultScanner             = deviceScanner{
		isMouse:       interception.IsMouse,
		isKeyboard:    interception.IsKeyboard,
		getHardwareID: interception.GetHardwareID,
	}
)

// Init initializes the Interception context and finds devices.
// It loads the DLL, creates a context, and scans for valid non-DD mouse and keyboard devices.
func Init() error {
	initMutex.Lock()
	defer initMutex.Unlock()

	if initialized {
		return nil
	}

	if err := interceptionLoad(); err != nil {
		return err
	}

	c := interceptionCreateContext()
	if c == 0 {
		interceptionUnload()
		return ErrDriverNotInstalled
	}

	selMouse, selKbd, err := selectDevices(c, defaultScanner)
	if err != nil {
		interceptionDestroyContext(c)
		interceptionUnload()
		ctx = 0
		mouseDev = 0
		keyboardDev = 0
		return err
	}

	ctx = c
	mouseDev = selMouse
	keyboardDev = selKbd
	initialized = true
	return nil
}

// selectDevices scans slots 1..MaxInterceptionDevices and selects the first valid
// non-DD mouse and keyboard while rejecting empty, virtual DD, and unreadable slots.
func selectDevices(c interception.Context, scan deviceScanner) (interception.Device, interception.Device, error) {
	var (
		selMouse, selKbd           interception.Device
		sawDDMouse, sawDDKbd       bool
		mouseQueryErr, kbdQueryErr error
	)

	for i := 1; i <= MaxInterceptionDevices; i++ {
		dev := interception.Device(i)
		needMouse := selMouse == 0 && scan.isMouse(dev)
		needKbd := selKbd == 0 && scan.isKeyboard(dev)
		if !needMouse && !needKbd {
			if selMouse != 0 && selKbd != 0 {
				break
			}
			continue
		}

		hwids, qErr := scan.getHardwareID(c, dev)
		if qErr != nil {
			// Never accept an unverified/malformed slot as a physical device.
			if needMouse && mouseQueryErr == nil {
				mouseQueryErr = qErr
			}
			if needKbd && kbdQueryErr == nil {
				kbdQueryErr = qErr
			}
			continue
		}
		if len(hwids) == 0 {
			// Empty slot (0 returned bytes).
			continue
		}
		if isVirtualDDHardwareID(hwids) {
			if needMouse {
				sawDDMouse = true
			}
			if needKbd {
				sawDDKbd = true
			}
			continue
		}

		if needMouse {
			selMouse = dev
		}
		if needKbd {
			selKbd = dev
		}
		if selMouse != 0 && selKbd != 0 {
			break
		}
	}

	if selMouse == 0 && selKbd == 0 {
		if err := firstError(mouseQueryErr, kbdQueryErr); err != nil {
			return 0, 0, fmt.Errorf("interception hardware id check failed: %w", err)
		}
		if sawDDMouse || sawDDKbd {
			return 0, 0, fmt.Errorf("%w: only virtual DD devices found", ErrDevicesUnavailable)
		}
		return 0, 0, ErrDevicesUnavailable
	}
	if selMouse == 0 {
		if mouseQueryErr != nil {
			return 0, 0, fmt.Errorf("interception mouse hardware id check failed: %w", mouseQueryErr)
		}
		if sawDDMouse {
			return 0, 0, fmt.Errorf("%w: only virtual DD mouse devices found", ErrDevicesUnavailable)
		}
	}
	if selKbd == 0 {
		if kbdQueryErr != nil {
			return 0, 0, fmt.Errorf("interception keyboard hardware id check failed: %w", kbdQueryErr)
		}
		if sawDDKbd {
			return 0, 0, fmt.Errorf("%w: only virtual DD keyboard devices found", ErrDevicesUnavailable)
		}
	}

	return selMouse, selKbd, nil
}

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// isVirtualDDHardwareID reports whether any PnP hardware ID identifies the
// ddhid63340 virtual HID device or its parent enumerator.
func isVirtualDDHardwareID(hwids []string) bool {
	for _, id := range hwids {
		s := strings.ToLower(strings.TrimSpace(id))
		if s == `hid\ddhid63340` || strings.HasPrefix(s, `hid\ddhid63340&`) ||
			s == `ddhid63340` || strings.HasPrefix(s, `ddhid63340\`) {
			return true
		}
	}
	return false
}

// Close destroys the Interception context and unloads the DLL.
// It ensures that no further input operations can be performed.
func Close() error {
	initMutex.Lock()
	defer initMutex.Unlock()

	if !initialized {
		return nil
	}

	if ctx != 0 {
		interceptionDestroyContext(ctx)
		ctx = 0
	}
	mouseDev = 0
	keyboardDev = 0
	initialized = false

	interceptionUnload()
	return nil
}

// EnsureInit checks if the HID backend is initialized, and initializes it if not.
func EnsureInit() error {
	initMutex.RLock()
	if initialized {
		initMutex.RUnlock()
		return nil
	}
	initMutex.RUnlock()
	return Init()
}

func humanSleep(base int) {
	maxJitter := base / 3
	if maxJitter == 0 {
		maxJitter = 1
	}
	jitter := rand.Intn(maxJitter*2+1) - maxJitter

	duration := base + jitter
	if duration < 0 {
		duration = 0
	}
	time.Sleep(time.Duration(duration) * time.Millisecond)
}

func acquireSlot(slot *interception.Device) (interception.Context, interception.Device, func(), error) {
	if err := EnsureInit(); err != nil {
		return 0, 0, nil, err
	}
	initMutex.RLock()
	if !initialized {
		initMutex.RUnlock()
		return 0, 0, nil, fmt.Errorf("hid backend closed")
	}
	dev := *slot
	if dev == 0 {
		initMutex.RUnlock()
		return 0, 0, nil, ErrDevicesUnavailable
	}
	return ctx, dev, initMutex.RUnlock, nil
}

// Helper to acquire lock and return handles.
// Caller MUST call unlock() when done.
func acquireMouse() (interception.Context, interception.Device, func(), error) {
	return acquireSlot(&mouseDev)
}

func acquireKeyboard() (interception.Context, interception.Device, func(), error) {
	return acquireSlot(&keyboardDev)
}

// -----------------------------------------------------------------------------
// Mouse
// -----------------------------------------------------------------------------

func abs(n int32) int32 {
	if n < 0 {
		return -n
	}
	return n
}

// insideVirtualBounds reports whether (x, y) is inside the half-open virtual
// desktop rectangle [Left, Right) x [Top, Bottom).
func insideVirtualBounds(x, y int32, b screen.Rect) bool {
	return x >= b.Left && x < b.Right && y >= b.Top && y < b.Bottom
}

// Move simulates mouse movement using the human-like trajectory.
func Move(x, y int32) error { return interceptionMotion().move(x, y) }

// Click moves once, then performs the left button phase under a new lock.
func Click(x, y int32) error { return interceptionMotion().click(x, y, motion.Left) }

// ClickRight moves once, then performs the right button phase under a new lock.
func ClickRight(x, y int32) error { return interceptionMotion().click(x, y, motion.Right) }

// ClickMiddle moves once, then performs the middle button phase under a new lock.
func ClickMiddle(x, y int32) error { return interceptionMotion().click(x, y, motion.Middle) }

// DoubleClick preserves Interception's system positioning in the second phase.
func DoubleClick(x, y int32) error { return interceptionMotion().doubleClick(x, y) }

// MoveRelative sends one relative mouse stroke.
func MoveRelative(dx, dy int32) error {
	lCtx, lDev, unlock, err := acquireMouse()
	if err != nil {
		return err
	}
	defer unlock()

	stroke := interception.MouseStroke{
		Flags: interception.MouseFlagMoveRelative,
		X:     dx,
		Y:     dy,
	}
	return interception.SendMouse(lCtx, lDev, &stroke)
}

// MoveExact sends one absolute stroke over the virtual desktop, waits 12ms,
// and verifies the result within one pixel in the process coordinate space.
func MoveExact(targetX, targetY int32) error {
	bounds := screen.VirtualBounds()
	width := int64(bounds.Right - bounds.Left)
	height := int64(bounds.Bottom - bounds.Top)
	if width < 2 || height < 2 {
		return fmt.Errorf("virtual desktop too small for absolute movement")
	}
	if !insideVirtualBounds(targetX, targetY, bounds) {
		return ErrOutsideVirtualDesktop
	}

	nx := int32((int64(targetX) - int64(bounds.Left)) * 65535 / (width - 1))
	ny := int32((int64(targetY) - int64(bounds.Top)) * 65535 / (height - 1))

	lCtx, lDev, unlock, err := acquireMouse()
	if err != nil {
		return err
	}
	defer unlock()

	stroke := interception.MouseStroke{
		State: 0,
		Flags: interception.MouseFlagMoveAbsolute | interception.MouseFlagMoveVirtualDesktop,
		X:     nx,
		Y:     ny,
	}
	if err := interception.SendMouse(lCtx, lDev, &stroke); err != nil {
		return err
	}

	time.Sleep(12 * time.Millisecond)
	curX, curY, err := window.GetCursorPos()
	if err != nil {
		return err
	}
	if abs(targetX-curX) > 1 || abs(targetY-curY) > 1 {
		return ErrMoveNotConverged
	}
	return nil
}

// Button sends one button stroke without moving the cursor.
// state is an Interception mouse state such as MouseStateLeftDown.
// The value is not validated.
func Button(state uint16) error {
	lCtx, lDev, unlock, err := acquireMouse()
	if err != nil {
		return err
	}
	defer unlock()

	stroke := interception.MouseStroke{State: state}
	return interception.SendMouse(lCtx, lDev, &stroke)
}

const (
	minScrollDelta = -32768
	maxScrollDelta = 32767
)

func wheelDelta(delta int32) (int16, error) {
	if delta < minScrollDelta || delta > maxScrollDelta {
		return 0, ErrInvalidScrollDelta
	}
	return int16(delta), nil
}

// Scroll simulates a vertical mouse wheel scroll.
func Scroll(delta int32) error {
	if delta == 0 {
		return nil
	}
	rolling, err := wheelDelta(delta)
	if err != nil {
		return err
	}

	lCtx, lDev, unlock, err := acquireMouse()
	if err != nil {
		return err
	}
	defer unlock()

	stroke := interception.MouseStroke{
		State:   interception.MouseStateWheel,
		Rolling: rolling,
	}
	if err := interception.SendMouse(lCtx, lDev, &stroke); err != nil {
		return err
	}
	return nil
}

// -----------------------------------------------------------------------------
// Keyboard
// -----------------------------------------------------------------------------

// keyStroke decodes a composite keyboard.Key into an Interception stroke.
// The E0 flag is applied to both down and up events. See KeyDown.
func keyStroke(key uint16, up bool) (interception.KeyStroke, error) {
	scan, extended, ok := keyboard.Key(key).Decode()
	if !ok {
		return interception.KeyStroke{}, keyboard.ErrUnsupportedKey
	}
	state := uint16(interception.KeyStateDown)
	if up {
		state = interception.KeyStateUp
	}
	if extended {
		state |= interception.KeyStateE0
	}
	return interception.KeyStroke{
		Code:  uint16(scan),
		State: state,
	}, nil
}

// KeyDown sends a key-down stroke for a keyboard.Key value.
//
// The argument is the composite encoding, not a raw scan code: the low 8
// bits are the Set 1 scan code and the high byte is 0xE0 when the key is
// extended. Passing 0x48 selects numpad 8; arrow up is 0xE048.
func KeyDown(key uint16) error {
	s, err := keyStroke(key, false)
	if err != nil {
		return err
	}

	lCtx, lDev, unlock, err := acquireKeyboard()
	if err != nil {
		return err
	}
	defer unlock()

	if err := interception.SendKey(lCtx, lDev, &s); err != nil {
		return err
	}
	return nil
}

// KeyUp sends a key-up stroke for a keyboard.Key value.
// The argument uses the same composite encoding as KeyDown.
func KeyUp(key uint16) error {
	s, err := keyStroke(key, true)
	if err != nil {
		return err
	}

	lCtx, lDev, unlock, err := acquireKeyboard()
	if err != nil {
		return err
	}
	defer unlock()

	if err := interception.SendKey(lCtx, lDev, &s); err != nil {
		return err
	}
	return nil
}

// Press simulates a key press (down then up) for a keyboard.Key value.
// The argument uses the same composite encoding as KeyDown.
func Press(key uint16) error {
	// KeyDown and KeyUp will acquire/release locks individually.
	// This is safe.
	if err := KeyDown(key); err != nil {
		return err
	}
	humanSleep(40)
	return KeyUp(key)
}
