package winput

import (
	"errors"
	"fmt"
	"time"
	"unsafe"

	"github.com/rpdg/winput/window"
)

// Bound each activation/restore confirmation, rather than guessing a sleep.
// A restore reserves half its budget for draining a possibly pending target
// activation, leaving the rest to request and confirm the previous foreground.
const (
	foregroundTransitionTimeout = 500 * time.Millisecond
	foregroundPollInterval      = 10 * time.Millisecond
)

var (
	foregroundNow       = time.Now
	foregroundWaitSleep = time.Sleep
	foregroundSync      = syncForegroundWindow
)

// syncForegroundWindow is the WM_NULL acknowledgement described at
// https://devblogs.microsoft.com/oldnewthing/20161118-00/?p=94745 . A successful
// SetForegroundWindow can precede the target queue processing its activation.
// Do not attach input queues or synthesize input to force that activation.
func syncForegroundWindow(hwnd uintptr, timeout time.Duration) error {
	if timeout <= 0 {
		return fmt.Errorf("foreground acknowledgement deadline expired")
	}
	tid, _, _ := window.ProcGetWindowThreadProcessId.Call(hwnd, 0)
	if tid == 0 {
		return ErrWindowGone
	}
	current, _, _ := window.ProcGetCurrentThreadId.Call()
	if tid == current {
		// Activation on the calling thread is synchronous. Do not invoke its
		// window procedure again: SendMessageTimeout ignores same-queue timeouts.
		return nil
	}
	const (
		wmNull          = 0
		smtoBlock       = 0x0001
		smtoAbortIfHung = 0x0002
		smtoErrorOnExit = 0x0020
	)
	millis := (timeout + time.Millisecond - 1) / time.Millisecond
	var result uintptr
	r, _, err := window.ProcSendMessageTimeoutW.Call(hwnd, wmNull, 0, 0,
		smtoBlock|smtoAbortIfHung|smtoErrorOnExit, uintptr(millis), uintptr(unsafe.Pointer(&result)))
	if r == 0 {
		// This API can fail with errno 0; the return value is authoritative.
		return fmt.Errorf("foreground queue acknowledgement failed or timed out: %v", err)
	}
	return nil
}

func confirmHotkeyForeground(hwnd uintptr, deadline time.Time) error {
	if inputAncestorRoot(hwnd) == 0 {
		return ErrWindowGone
	}
	remaining := deadline.Sub(foregroundNow())
	if remaining <= 0 {
		return fmt.Errorf("foreground confirmation deadline expired")
	}
	// A matching snapshot alone is insufficient when activation is pending.
	if err := foregroundSync(hwnd, remaining); err != nil {
		return err
	}
	for {
		if deadline.Sub(foregroundNow()) <= 0 {
			return fmt.Errorf("foreground confirmation deadline expired")
		}
		if inputAncestorRoot(hwnd) == 0 {
			return ErrWindowGone
		}
		if inputForegroundWindow() == hwnd {
			return nil
		}
		remaining = deadline.Sub(foregroundNow())
		if remaining <= 0 {
			return fmt.Errorf("window did not become foreground before deadline")
		}
		pause := foregroundPollInterval
		if remaining < pause {
			pause = remaining
		}
		foregroundWaitSleep(pause)
	}
}

func restoreHotkeyForeground(target, previous uintptr) error {
	deadline := foregroundNow().Add(foregroundTransitionTimeout)
	var errs []error
	// Always drain the target before restoring, including activation errors
	// and the case where GetForegroundWindow still reports previous. Otherwise
	// a late target activation could undo an apparently successful restore.
	if err := foregroundSync(target, deadline.Sub(foregroundNow())/2); err != nil {
		errs = append(errs, fmt.Errorf("%w: target activation completion unconfirmed; a delayed foreground switch may still occur: %v", ErrPermissionDenied, err))
	}
	if err := inputSetForeground(previous); err != nil {
		errs = append(errs, fmt.Errorf("%w: failed to restore foreground window: %v", ErrPermissionDenied, err))
	}
	// Confirm even when the request reports an error: it may have side effects.
	// Never turn a failed target acknowledgement into a success merely because
	// previous is foreground now. A timeout does not cancel an OS activation.
	if err := confirmHotkeyForeground(previous, deadline); err != nil {
		errs = append(errs, fmt.Errorf("%w: restored foreground was not confirmed: %v", ErrPermissionDenied, err))
	}
	return errors.Join(errs...)
}
