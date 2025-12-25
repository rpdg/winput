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

func SetLibraryPath(path string) {
	interception.SetLibraryPath(path)
}

// Use a local random source instead of global rand
var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

var (
	ctx         interception.Context
	mouseDev    interception.Device
	keyboardDev interception.Device
	initialized bool
	initMutex   sync.RWMutex
)

// Init initializes the Interception context and finds devices.
func Init() error {
	initMutex.Lock()
	defer initMutex.Unlock()

	if initialized {
		return nil
	}

	if err := interception.Load(); err != nil {
		return err // Will be wrapped by winput
	}

	ctx = interception.CreateContext()
	if ctx == 0 {
		return ErrDriverNotInstalled
	}

	// Simple device discovery: iterate 1..20
	for i := 1; i <= 20; i++ {
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
		ctx = 0
		return fmt.Errorf("no interception devices found")
	}

	initialized = true
	return nil
}

// Close destroys the Interception context and unloads the DLL.
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

// EnsureInit checks initialization state.
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
	// Base +/- Jitter (max base/3)
	maxJitter := base / 3
	if maxJitter == 0 {
		maxJitter = 1
	}
	jitter := rng.Intn(maxJitter*2+1) - maxJitter // -maxJitter to +maxJitter
	
	duration := base + jitter
	if duration < 0 {
		duration = 0
	}
	time.Sleep(time.Duration(duration) * time.Millisecond)
}

// Helper to safely get context and device for operations
func getMouse() (interception.Context, interception.Device, error) {
	if err := EnsureInit(); err != nil {
		return 0, 0, err
	}
	initMutex.RLock()
	defer initMutex.RUnlock()
	if !initialized {
		return 0, 0, fmt.Errorf("hid backend closed")
	}
	return ctx, mouseDev, nil
}

func getKeyboard() (interception.Context, interception.Device, error) {
	if err := EnsureInit(); err != nil {
		return 0, 0, err
	}
	initMutex.RLock()
	defer initMutex.RUnlock()
	if !initialized {
		return 0, 0, fmt.Errorf("hid backend closed")
	}
	return ctx, keyboardDev, nil
}

// -----------------------------------------------------------------------------
// Mouse
// -----------------------------------------------------------------------------

func Move(targetX, targetY int32) error {
	lCtx, lDev, err := getMouse()
	if err != nil {
		return err
	}

	cx, cy, err := window.GetCursorPos()
	if err != nil {
		return err
	}

	steps := 20
	for i := 1; i <= steps; i++ {
		nextX := cx + (targetX-cx)*int32(i)/int32(steps)
		nextY := cy + (targetY-cy)*int32(i)/int32(steps)

		curX, curY, err := window.GetCursorPos()
		if err != nil {
			return err
		}

		dx := nextX - curX
		dy := nextY - curY

		if i < steps {
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
		time.Sleep(5 * time.Millisecond)
	}
	return nil
}

func Click(x, y int32) error {
	// Move handles its own context retrieval
	if err := Move(x, y); err != nil {
		return err
	}
	
	lCtx, lDev, err := getMouse()
	if err != nil {
		return err
	}

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

func ClickRight(x, y int32) error {
	if err := Move(x, y); err != nil {
		return err
	}
	
	lCtx, lDev, err := getMouse()
	if err != nil {
		return err
	}

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

func ClickMiddle(x, y int32) error {
	if err := Move(x, y); err != nil {
		return err
	}
	
	lCtx, lDev, err := getMouse()
	if err != nil {
		return err
	}

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

func DoubleClick(x, y int32) error {
	if err := Click(x, y); err != nil {
		return err
	}
	humanSleep(80)
	return Click(x, y)
}

func Scroll(delta int32) error {
	lCtx, lDev, err := getMouse()
	if err != nil {
		return err
	}

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

func KeyDown(scanCode uint16) error {
	lCtx, lDev, err := getKeyboard()
	if err != nil {
		return err
	}

	s := interception.KeyStroke{
		Code:  scanCode,
		State: interception.KeyStateDown,
	}
	if err := interception.SendKey(lCtx, lDev, &s); err != nil {
		return err
	}
	return nil
}

func KeyUp(scanCode uint16) error {
	lCtx, lDev, err := getKeyboard()
	if err != nil {
		return err
	}

	s := interception.KeyStroke{
		Code:  scanCode,
		State: interception.KeyStateUp,
	}
	if err := interception.SendKey(lCtx, lDev, &s); err != nil {
		return err
	}
	return nil
}

func Press(scanCode uint16) error {
	// Note: We retrieve context inside KeyDown/KeyUp individually.
	// This is fine and thread-safe.
	if err := KeyDown(scanCode); err != nil {
		return err
	}
	humanSleep(40)
	return KeyUp(scanCode)
}
