// Package motion contains the human trajectory and button timing algorithms.
// Callers own lifecycle locks and supply callbacks valid for just one phase.
package motion

import (
	"fmt"
	"time"
)

type Button uint8

const (
	Left Button = iota
	Right
	Middle
)

// Session is a callback bundle for a lifecycle phase already locked by its
// caller. Callbacks must not reacquire that lock or escape into another phase.
type Session struct {
	MoveRelative func(dx, dy int32) error
	Button       func(b Button, down bool) error
}

// Runtime supplies only the observations used by the algorithms. Production
// bindings retain the caller's cursor, time.After, time.Sleep and rand.Intn.
// Supply the callbacks used by the requested operation; unused ones are not
// validated, preserving error reachability at the original call sites.
type Runtime struct {
	Cursor func() (int32, int32, error)
	Sleep  func(time.Duration)
	Intn   func(int) int
	After  func(time.Duration) <-chan time.Time
}

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

// Move runs a trajectory while the caller holds the movement phase lock.
// notConverged is returned unchanged only after exhausting final corrections,
// so the adapter can retain its existing public error identity.
func Move(s Session, r Runtime, targetX, targetY int32, notConverged error) error {
	cx, cy, err := r.Cursor()
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
		if i > steps-5 && abs(dx) < 3 && abs(dy) < 3 {
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

		if err := s.MoveRelative(dx, dy); err != nil {
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

		if humanArrived(dx, dy) {
			return nil // Reached target within the human-path tolerance
		}

		// Micro-correction
		if err := s.MoveRelative(dx, dy); err != nil {
			return err
		}
	}

	// The fifth correction was not observed. Read the cursor once more before
	// deciding success; the loop above does not re-read after its last send.
	curX, curY, err := r.Cursor()
	if err != nil {
		return err
	}
	if humanArrived(targetX-curX, targetY-curY) {
		return nil
	}
	return notConverged
}
