// Package hid drives the Interception backend.
//
// The exported functions are not safe for concurrent use. Callers must
// serialize them. winput does this with its inputMutex.
package hid

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/rpdg/winput/hid/interception"
	"github.com/rpdg/winput/keyboard"
	"github.com/rpdg/winput/screen"
	"github.com/rpdg/winput/window"
)

var (
	ErrDriverNotInstalled    = errors.New("interception driver not installed or accessible")
	ErrMoveNotConverged      = errors.New("HID move did not converge")
	ErrOutsideVirtualDesktop = errors.New("target outside virtual desktop")
	ErrInvalidScrollDelta    = errors.New("scroll delta out of int16 range")
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
	jitter := rand.Intn(maxJitter*2+1) - maxJitter

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

// humanArrived is the arrival contract for MoveMouseTo's human-like path.
// Two pixels of drift are accepted; three pixels on either axis are not.
func humanArrived(dx, dy int32) bool {
	return abs(dx) < 3 && abs(dy) < 3
}

// insideVirtualBounds reports whether (x, y) is inside the half-open virtual
// desktop rectangle [Left, Right) x [Top, Bottom).
func insideVirtualBounds(x, y int32, b screen.Rect) bool {
	return x >= b.Left && x < b.Right && y >= b.Top && y < b.Bottom
}

// Move simulates mouse movement to the target screen coordinates using human-like trajectory.
func Move(targetX, targetY int32) error {
	bounds := screen.VirtualBounds()
	if !insideVirtualBounds(targetX, targetY, bounds) {
		return ErrOutsideVirtualDesktop
	}

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

	timeout := time.After(3 * time.Second) // Increased timeout for robustness

	// 1. Trajectory Loop
	for i := 1; i <= steps; i++ {
		select {
		case <-timeout:
			return fmt.Errorf("move timeout during trajectory")
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

		// Optimization: If very close (within jitter range), skip correction
		// to avoid oscillation near the target.
		if i > steps-5 && abs(dx) < 3 && abs(dy) < 3 {
			continue
		}

		// Apply jitter only if not the final few steps
		if i < steps-2 {
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

	// 2. Final Convergence (Critical for Click accuracy)
	// Even after the loop, we might be off by a few pixels due to jitter or async lag.
	// Force convergence.
	for retry := 0; retry < 5; retry++ {
		time.Sleep(20 * time.Millisecond) // Wait for OS to settle

		curX, curY, err := window.GetCursorPos()
		if err != nil {
			return err
		}

		dx := targetX - curX
		dy := targetY - curY

		if humanArrived(dx, dy) {
			return nil // Reached target within the human-path tolerance
		}

		// Micro-correction
		stroke := interception.MouseStroke{
			Flags: interception.MouseFlagMoveRelative,
			X:     dx,
			Y:     dy,
		}
		if err := interception.SendMouse(lCtx, lDev, &stroke); err != nil {
			return err
		}
	}

	// The fifth correction was not observed. Read the cursor once more before
	// deciding success; the loop above does not re-read after its last send.
	curX, curY, err := window.GetCursorPos()
	if err != nil {
		return err
	}
	if humanArrived(targetX-curX, targetY-curY) {
		return nil
	}
	return ErrMoveNotConverged
}

// releaseButton sends a button-up stroke once and retries once on failure.
// The first error is preserved when both attempts fail.
func releaseButton(ctx interception.Context, dev interception.Device, state uint16) error {
	up := interception.MouseStroke{State: state}
	if err := interception.SendMouse(ctx, dev, &up); err != nil {
		time.Sleep(6 * time.Millisecond)
		if err2 := interception.SendMouse(ctx, dev, &up); err2 != nil {
			return err
		}
	}
	return nil
}

// clickRaw performs a left click at current position without movement logic.
// Caller must hold the lock/context.
// minHold/maxHold define the duration (ms) the button remains pressed.
func clickRaw(ctx interception.Context, dev interception.Device, minHold, maxHold int) error {
	// Pre-click delay (muscle memory) - small jitter
	humanSleep(20 + rand.Intn(20))

	down := interception.MouseStroke{State: interception.MouseStateLeftDown}
	if err := interception.SendMouse(ctx, dev, &down); err != nil {
		return err
	}

	// Hold time
	hold := minHold
	if maxHold > minHold {
		hold += rand.Intn(maxHold - minHold)
	}
	time.Sleep(time.Duration(hold) * time.Millisecond)

	return releaseButton(ctx, dev, interception.MouseStateLeftUp)
}

// Click simulates a left mouse button click at the current cursor position.
// It triggers Move first to ensure correct context acquisition.
func Click(x, y int32) error {
	if err := Move(x, y); err != nil {
		return err
	}

	lCtx, lDev, unlock, err := acquireMouse()
	if err != nil {
		return err
	}
	defer unlock()

	// Stabilize after move
	// Move() now guarantees convergence, but we still need a muscle memory pause.
	time.Sleep(50 * time.Millisecond)

	// Normal click: hold 60-90ms
	return clickRaw(lCtx, lDev, 60, 90)
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

	time.Sleep(50 * time.Millisecond)

	down := interception.MouseStroke{State: interception.MouseStateRightDown}
	if err := interception.SendMouse(lCtx, lDev, &down); err != nil {
		return err
	}

	humanSleep(60)

	return releaseButton(lCtx, lDev, interception.MouseStateRightUp)
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

	time.Sleep(50 * time.Millisecond)

	down := interception.MouseStroke{State: interception.MouseStateMiddleDown}
	if err := interception.SendMouse(lCtx, lDev, &down); err != nil {
		return err
	}

	humanSleep(60)

	return releaseButton(lCtx, lDev, interception.MouseStateMiddleUp)
}

// DoubleClick simulates a left mouse button double-click at the current cursor position.
// It moves ONCE, then clicks twice rapidly and deterministically.
func DoubleClick(x, y int32) error {
	// 1. Move 到目标（保留你的 Move 保证轨迹与视觉一致）
	if err := Move(x, y); err != nil {
		return err
	}

	// 2. Acquire device
	lCtx, lDev, unlock, err := acquireMouse()
	if err != nil {
		return err
	}
	defer unlock()

	// 3. 强制设系统光标到精确目标（消除相对移动异步）
	// window 包中假定存在 ProcSetCursorPos (见你其它处用法)
	r, _, _ := window.ProcSetCursorPos.Call(uintptr(x), uintptr(y))
	if r == 0 {
		// 若 SetCursorPos 失败，仍可继续，但记录/返回错误更安全
		// 这里选择返回错误，让调用者显式处理
		return fmt.Errorf("SetCursorPos failed")
	}
	// 睡短候，以便系统合成与驱动稳定（一般 8~20ms 足够）
	time.Sleep(12 * time.Millisecond)

	// 4. 读取系统双击时间并选取稳健间隔（取三分之一，且不小于 30ms）
	r2, _, _ := window.ProcGetDoubleClickTime.Call()
	sysDc := time.Duration(r2) * time.Millisecond
	if sysDc == 0 {
		sysDc = 500 * time.Millisecond
	}
	interval := sysDc / 3
	if interval < 30*time.Millisecond {
		interval = 30 * time.Millisecond
	}
	hold := 25 * time.Millisecond // Down 保持时间

	down := interception.MouseStroke{State: interception.MouseStateLeftDown}
	up := interception.MouseStroke{State: interception.MouseStateLeftUp}

	// 单笔发送，失败后再试一次。两次都失败时留下第一次的错误。
	sendOnce := func(st *interception.MouseStroke) error {
		if err := interception.SendMouse(lCtx, lDev, st); err != nil {
			time.Sleep(6 * time.Millisecond)
			if err2 := interception.SendMouse(lCtx, lDev, st); err2 != nil {
				return err
			}
		}
		return nil
	}

	// 5. 原子双击序列（无随机、无 Move）
	// First Down/Up
	if err := sendOnce(&down); err != nil {
		return err
	}
	time.Sleep(hold)
	if err := sendOnce(&up); err != nil {
		return err
	}

	// Interval（严格且确定）
	time.Sleep(interval)

	// Second Down/Up
	if err := sendOnce(&down); err != nil {
		return err
	}
	time.Sleep(hold)
	if err := sendOnce(&up); err != nil {
		return err
	}

	return nil
}

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
