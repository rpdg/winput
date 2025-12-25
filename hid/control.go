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

func init() {
	rand.Seed(time.Now().UnixNano())
}

var (
	ctx         interception.Context
	mouseDev    interception.Device
	keyboardDev interception.Device
	initialized bool
	initMutex   sync.Mutex
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

// Close destroys the Interception context.
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
	return nil
}

// EnsureInit checks initialization state.
func EnsureInit() error {
	if initialized {
		return nil
	}
	return Init()
}

func humanSleep(base int) {
	jitter := rand.Intn(base/3 + 1)
	time.Sleep(time.Duration(base+jitter) * time.Millisecond)
}

// -----------------------------------------------------------------------------
// Mouse
// -----------------------------------------------------------------------------

func Move(targetX, targetY int32) error {
	if err := EnsureInit(); err != nil {
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
			dx += int32(rand.Intn(3) - 1)
			dy += int32(rand.Intn(3) - 1)
		}

		if dx == 0 && dy == 0 {
			continue
		}

		stroke := interception.MouseStroke{
			Flags: interception.MouseFlagMoveRelative,
			X:     dx,
			Y:     dy,
		}

		if err := interception.SendMouse(ctx, mouseDev, &stroke); err != nil {
			return err
		}
		time.Sleep(5 * time.Millisecond)
	}
	return nil
}

func Click(x, y int32) error {
	if err := Move(x, y); err != nil {
		return err
	}
	humanSleep(50)

	down := interception.MouseStroke{State: interception.MouseStateLeftDown}
	if err := interception.SendMouse(ctx, mouseDev, &down); err != nil {
		return err
	}

	humanSleep(60)

	up := interception.MouseStroke{State: interception.MouseStateLeftUp}
	if err := interception.SendMouse(ctx, mouseDev, &up); err != nil {
		return err
	}

	return nil
}

func ClickRight(x, y int32) error {
	if err := Move(x, y); err != nil {
		return err
	}
	humanSleep(50)

	down := interception.MouseStroke{State: interception.MouseStateRightDown}
	if err := interception.SendMouse(ctx, mouseDev, &down); err != nil {
		return err
	}

	humanSleep(60)

	up := interception.MouseStroke{State: interception.MouseStateRightUp}
	if err := interception.SendMouse(ctx, mouseDev, &up); err != nil {
		return err
	}
	return nil
}

func ClickMiddle(x, y int32) error {
	if err := Move(x, y); err != nil {
		return err
	}
	humanSleep(50)

	down := interception.MouseStroke{State: interception.MouseStateMiddleDown}
	if err := interception.SendMouse(ctx, mouseDev, &down); err != nil {
		return err
	}

	humanSleep(60)

	up := interception.MouseStroke{State: interception.MouseStateMiddleUp}
	if err := interception.SendMouse(ctx, mouseDev, &up); err != nil {
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
	if err := EnsureInit(); err != nil {
		return err
	}

	stroke := interception.MouseStroke{
		State:   interception.MouseStateWheel,
		Rolling: int16(delta),
	}
	if err := interception.SendMouse(ctx, mouseDev, &stroke); err != nil {
		return err
	}
	return nil
}

// -----------------------------------------------------------------------------
// Keyboard
// -----------------------------------------------------------------------------

func KeyDown(scanCode uint16) error {
	if err := EnsureInit(); err != nil {
		return err
	}
	s := interception.KeyStroke{
		Code:  scanCode,
		State: interception.KeyStateDown,
	}
	if err := interception.SendKey(ctx, keyboardDev, &s); err != nil {
		return err
	}
	return nil
}

func KeyUp(scanCode uint16) error {
	if err := EnsureInit(); err != nil {
		return err
	}
	s := interception.KeyStroke{
		Code:  scanCode,
		State: interception.KeyStateUp,
	}
	if err := interception.SendKey(ctx, keyboardDev, &s); err != nil {
		return err
	}
	return nil
}

func Press(scanCode uint16) error {
	if err := KeyDown(scanCode); err != nil {
		return err
	}
	humanSleep(40)
	return KeyUp(scanCode)
}
