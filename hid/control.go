package hid

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/rpdg/winput/hid/interception"
	"github.com/rpdg/winput/window"
)

var ErrDriverNotInstalled = errors.New("interception driver not installed or accessible")

// SetLibraryPath sets the custom path for the interception.dll library.
func SetLibraryPath(path string) {
	interception.SetLibraryPath(path)
}

const (
	MaxInterceptionDevices = 20
)

// Use a local random source instead of global rand
var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

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

// Init initializes the Interception context and finds devices.
// It loads the DLL, creates a context, and scans for mouse and keyboard devices.
func Init() error {
	initMutex.Lock()
	defer initMutex.Unlock()

	if initialized {
		return nil
	}

	if err := interception.Load(); err != nil {
		return err
	}

	ctx = interception.CreateContext()
	if ctx == 0 {
		interception.Unload()
		return ErrDriverNotInstalled
	}

	// Device discovery
	for i := 1; i <= MaxInterceptionDevices; i++ {
		dev := interception.Device(i)
		if interception.IsMouse(dev) && mouseDev == 0 {
			mouseDev = dev
		}
		if interception.IsKeyboard(dev) && keyboardDev == 0 {
			keyboardDev = dev
		}
	}

	if mouseDev == 0 && keyboardDev == 0 {
		interception.DestroyContext(ctx)
		interception.Unload()
		ctx = 0
		return fmt.Errorf("no interception devices found")
	}

	initialized = true
	return nil
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
		interception.DestroyContext(ctx)
		ctx = 0
	}
	mouseDev = 0
	keyboardDev = 0
	initialized = false

	interception.Unload()
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
	jitter := rng.Intn(maxJitter*2+1) - maxJitter

	duration := base + jitter
	if duration < 0 {
		duration = 0
	}
	time.Sleep(time.Duration(duration) * time.Millisecond)
}

// Helper to acquire lock and return handles.
// Caller MUST call unlock() when done.
func acquireMouse() (interception.Context, interception.Device, func(), error) {
	if err := EnsureInit(); err != nil {
		return 0, 0, nil, err
	}
	initMutex.RLock()
	if !initialized {
		initMutex.RUnlock()
		return 0, 0, nil, fmt.Errorf("hid backend closed")
	}
	return ctx, mouseDev, initMutex.RUnlock, nil
}

func acquireKeyboard() (interception.Context, interception.Device, func(), error) {
	if err := EnsureInit(); err != nil {
		return 0, 0, nil, err
	}
	initMutex.RLock()
	if !initialized {
		initMutex.RUnlock()
		return 0, 0, nil, fmt.Errorf("hid backend closed")
	}
	return ctx, keyboardDev, initMutex.RUnlock, nil
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

func max(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

// Move simulates mouse movement to the target screen coordinates using human-like trajectory.
func Move(targetX, targetY int32) error {
	lCtx, lDev, unlock, err := acquireMouse()
	if err != nil {
		return err
	}
	defer unlock()

	cx, cy, err := window.GetCursorPos()
	if err != nil {
		return err
	}

	dxTotal := abs(targetX - cx)
	dyTotal := abs(targetY - cy)
	maxDist := max(dxTotal, dyTotal)

	// Adaptive steps calculation
	var steps int
	switch {
	case maxDist < 100:
		steps = int(maxDist / 5) // Fine control
		if steps < 5 {
			steps = 5
		}
	case maxDist < 500:
		steps = 20
	case maxDist < 1000:
		steps = 30
	default:
		steps = 40 // Capped for speed
	}

	timeout := time.After(2 * time.Second)

	for i := 1; i <= steps; i++ {
		select {
		case <-timeout:
			return fmt.Errorf("move timeout")
		default:
		}

		nextX := cx + (targetX-cx)*int32(i)/int32(steps)
		nextY := cy + (targetY-cy)*int32(i)/int32(steps)

		curX, curY, err := window.GetCursorPos()
		if err != nil {
			return err
		}

		dx := nextX - curX
		dy := nextY - curY

		if i > steps-5 && abs(dx) < 3 && abs(dy) < 3 {
			continue
		}

		if i < steps-2 {
			dx += int32(rng.Intn(3) - 1)
			dy += int32(rng.Intn(3) - 1)
		}

		if dx == 0 && dy == 0 {
			continue
		}

		stroke := interception.MouseStroke{
			Flags: interception.MouseFlagMoveRelative,
			X:     dx,
			Y:     dy,
		}

		if err := interception.SendMouse(lCtx, lDev, &stroke); err != nil {
			return err
		}

		// Adaptive sleep
		sleepTime := 5
		if steps > 30 {
			sleepTime = 3 // Faster for long distances
		}
		time.Sleep(time.Duration(sleepTime) * time.Millisecond)
	}
	return nil
}

// Click simulates a left mouse button click at the current cursor position.
// It triggers Move first to ensure correct context acquisition.
func Click(x, y int32) error {
	// Move handles locking internally, but we need lock for Click actions too.
	// It's okay to release lock between Move and Click, or we can hold it.
	// For simplicity, we let Move do its thing (acquire/release), then we acquire again.
	if err := Move(x, y); err != nil {
		return err
	}

	lCtx, lDev, unlock, err := acquireMouse()
	if err != nil {
		return err
	}
	defer unlock()

	humanSleep(50)

	down := interception.MouseStroke{State: interception.MouseStateLeftDown}
	if err := interception.SendMouse(lCtx, lDev, &down); err != nil {
		return err
	}

	humanSleep(60)

	up := interception.MouseStroke{State: interception.MouseStateLeftUp}
	if err := interception.SendMouse(lCtx, lDev, &up); err != nil {
		return err
	}

	return nil
}

// ClickRight simulates a right mouse button click at the current cursor position.
func ClickRight(x, y int32) error {
	if err := Move(x, y); err != nil {
		return err
	}

	lCtx, lDev, unlock, err := acquireMouse()
	if err != nil {
		return err
	}
	defer unlock()

	humanSleep(50)

	down := interception.MouseStroke{State: interception.MouseStateRightDown}
	if err := interception.SendMouse(lCtx, lDev, &down); err != nil {
		return err
	}

	humanSleep(60)

	up := interception.MouseStroke{State: interception.MouseStateRightUp}
	if err := interception.SendMouse(lCtx, lDev, &up); err != nil {
		return err
	}
	return nil
}

// ClickMiddle simulates a middle mouse button click at the current cursor position.
func ClickMiddle(x, y int32) error {
	if err := Move(x, y); err != nil {
		return err
	}

	lCtx, lDev, unlock, err := acquireMouse()
	if err != nil {
		return err
	}
	defer unlock()

	humanSleep(50)

	down := interception.MouseStroke{State: interception.MouseStateMiddleDown}
	if err := interception.SendMouse(lCtx, lDev, &down); err != nil {
		return err
	}

	humanSleep(60)

	up := interception.MouseStroke{State: interception.MouseStateMiddleUp}
	if err := interception.SendMouse(lCtx, lDev, &up); err != nil {
		return err
	}
	return nil
}

// DoubleClick simulates a left mouse button double-click at the current cursor position.
func DoubleClick(x, y int32) error {
	if err := Click(x, y); err != nil {
		return err
	}
	humanSleep(80)
	return Click(x, y)
}

// Scroll simulates a vertical mouse wheel scroll.
func Scroll(delta int32) error {
	lCtx, lDev, unlock, err := acquireMouse()
	if err != nil {
		return err
	}
	defer unlock()

	stroke := interception.MouseStroke{
		State:   interception.MouseStateWheel,
		Rolling: int16(delta),
	}
	if err := interception.SendMouse(lCtx, lDev, &stroke); err != nil {
		return err
	}
	return nil
}

// -----------------------------------------------------------------------------
// Keyboard
// -----------------------------------------------------------------------------

// KeyDown simulates a key down event for the specified scan code.
func KeyDown(scanCode uint16) error {
	lCtx, lDev, unlock, err := acquireKeyboard()
	if err != nil {
		return err
	}
	defer unlock()

	s := interception.KeyStroke{
		Code:  scanCode,
		State: interception.KeyStateDown,
	}
	if err := interception.SendKey(lCtx, lDev, &s); err != nil {
		return err
	}
	return nil
}

// KeyUp simulates a key up event for the specified scan code.
func KeyUp(scanCode uint16) error {
	lCtx, lDev, unlock, err := acquireKeyboard()
	if err != nil {
		return err
	}
	defer unlock()

	s := interception.KeyStroke{
		Code:  scanCode,
		State: interception.KeyStateUp,
	}
	if err := interception.SendKey(lCtx, lDev, &s); err != nil {
		return err
	}
	return nil
}

// Press simulates a key press (down then up) for the specified scan code.
func Press(scanCode uint16) error {
	// KeyDown and KeyUp will acquire/release locks individually.
	// This is safe.
	if err := KeyDown(scanCode); err != nil {
		return err
	}
	humanSleep(40)
	return KeyUp(scanCode)
}