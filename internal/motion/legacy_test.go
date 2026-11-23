package motion

// Frozen reference extracted from hid/control.go at
// eb0022f49b5735662bc2b063e6ca453ca2599c59 (SHA256
// c2a1d21af25c49cac8ba5b725d15b3cf64bd6c965a6ac5e28d3bf211e6c18aae).
// Test-only algorithm-effects reference: the old loops and retry branches
// remain independent of the new helpers. Effect/type substitutions compare
// callback-level movement/button effects, timing, randomness and errors, not
// Interception wire fields. MoveRelative observes only stroke.X/Y.
import (
	"fmt"
	"time"
)

type legacyStroke struct {
	Flags, State uint16
	X, Y         int32
}

const (
	legacyLeftDown uint16 = 1
	legacyLeftUp   uint16 = 2
)

func legacySend(s Session, st *legacyStroke) error {
	return s.Button(Left, st.State == legacyLeftDown)
}
func legacyAbs(n int32) int32 {
	if n < 0 {
		return -n
	}
	return n
}

func legacyMax(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

// humanArrived is the arrival contract for MoveMouseTo's human-like path.
// Two pixels of drift are accepted; three pixels on either axis are not.
func legacyArrived(dx, dy int32) bool {
	return legacyAbs(dx) < 3 && legacyAbs(dy) < 3
}

func legacyHumanSleep(r Runtime, base int) {
	maxJitter := base / 3
	if maxJitter == 0 {
		maxJitter = 1
	}
	jitter := r.Intn(maxJitter*2+1) - maxJitter

	duration := base + jitter
	if duration < 0 {
		duration = 0
	}
	r.Sleep(time.Duration(duration) * time.Millisecond)
}

func legacyMove(s Session, r Runtime, targetX, targetY int32, notConverged error) error {
	cx, cy, err := r.Cursor()
	if err != nil {
		return err
	}

	dxTotal := legacyAbs(targetX - cx)
	dyTotal := legacyAbs(targetY - cy)
	maxDist := legacyMax(dxTotal, dyTotal)

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

	timeout := r.After(3 * time.Second) // Increased timeout for robustness

	// 1. Trajectory Loop
	for i := 1; i <= steps; i++ {
		select {
		case <-timeout:
			return fmt.Errorf("move timeout during trajectory")
		default:
		}

		nextX := cx + (targetX-cx)*int32(i)/int32(steps)
		nextY := cy + (targetY-cy)*int32(i)/int32(steps)

		curX, curY, err := r.Cursor()
		if err != nil {
			return err
		}

		dx := nextX - curX
		dy := nextY - curY

		// Optimization: If very close (within jitter range), skip correction
		// to avoid oscillation near the target.
		if i > steps-5 && legacyAbs(dx) < 3 && legacyAbs(dy) < 3 {
			continue
		}

		// Apply jitter only if not the final few steps
		if i < steps-2 {
			dx += int32(r.Intn(3) - 1)
			dy += int32(r.Intn(3) - 1)
		}

		if dx == 0 && dy == 0 {
			continue
		}

		stroke := legacyStroke{
			Flags: 0,
			X:     dx,
			Y:     dy,
		}

		if err := s.MoveRelative(stroke.X, stroke.Y); err != nil {
			return err
		}

		// Adaptive sleep
		sleepTime := 5
		if steps > 30 {
			sleepTime = 3 // Faster for long distances
		}
		r.Sleep(time.Duration(sleepTime) * time.Millisecond)
	}

	// 2. Final Convergence (Critical for Click accuracy)
	// Even after the loop, we might be off by a few pixels due to jitter or async lag.
	// Force convergence.
	for retry := 0; retry < 5; retry++ {
		r.Sleep(20 * time.Millisecond) // Wait for OS to settle

		curX, curY, err := r.Cursor()
		if err != nil {
			return err
		}

		dx := targetX - curX
		dy := targetY - curY

		if legacyArrived(dx, dy) {
			return nil // Reached target within the human-path tolerance
		}

		// Micro-correction
		stroke := legacyStroke{
			Flags: 0,
			X:     dx,
			Y:     dy,
		}
		if err := s.MoveRelative(stroke.X, stroke.Y); err != nil {
			return err
		}
	}

	// The fifth correction was not observed. Read the cursor once more before
	// deciding success; the loop above does not re-read after its last send.
	curX, curY, err := r.Cursor()
	if err != nil {
		return err
	}
	if legacyArrived(targetX-curX, targetY-curY) {
		return nil
	}
	return notConverged
}
func legacyRelease(s Session, r Runtime, state uint16) error {
	up := legacyStroke{State: state}
	if err := legacySend(s, &up); err != nil {
		r.Sleep(6 * time.Millisecond)
		if err2 := legacySend(s, &up); err2 != nil {
			return err
		}
	}
	return nil
}

func legacyClickRaw(s Session, r Runtime, minHold, maxHold int) error {
	// Pre-click delay (muscle memory) - small jitter
	legacyHumanSleep(r, 20+r.Intn(20))

	down := legacyStroke{State: legacyLeftDown}
	if err := legacySend(s, &down); err != nil {
		return err
	}

	// Hold time
	hold := minHold
	if maxHold > minHold {
		hold += r.Intn(maxHold - minHold)
	}
	r.Sleep(time.Duration(hold) * time.Millisecond)

	return legacyRelease(s, r, legacyLeftUp)
}

func legacyClick(s Session, r Runtime, b Button) error {
	r.Sleep(50 * time.Millisecond)
	if b == Left {
		return legacyClickRaw(s, r, 60, 90)
	}
	if err := s.Button(b, true); err != nil {
		return err
	}
	legacyHumanSleep(r, 60)
	if err := s.Button(b, false); err != nil {
		r.Sleep(6 * time.Millisecond)
		if err2 := s.Button(b, false); err2 != nil {
			return err
		}
	}
	return nil
}
func legacyDouble(s Session, r Runtime, interval time.Duration) error {
	hold := 25 * time.Millisecond // Down 保持时间

	down := legacyStroke{State: legacyLeftDown}
	up := legacyStroke{State: legacyLeftUp}

	// 单笔发送，失败后再试一次。两次都失败时留下第一次的错误。
	sendOnce := func(st *legacyStroke) error {
		if err := legacySend(s, st); err != nil {
			r.Sleep(6 * time.Millisecond)
			if err2 := legacySend(s, st); err2 != nil {
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
	r.Sleep(hold)
	if err := sendOnce(&up); err != nil {
		return err
	}

	// Interval（严格且确定）
	r.Sleep(interval)

	// Second Down/Up
	if err := sendOnce(&down); err != nil {
		return err
	}
	r.Sleep(hold)
	if err := sendOnce(&up); err != nil {
		return err
	}

	return nil
}
