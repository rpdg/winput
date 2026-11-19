package winput

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	fgPrevious = uintptr(99)
	fgTarget   = uintptr(7)
	fgOther    = uintptr(88)
)

type foregroundFixture struct {
	t        *testing.T
	now      time.Time
	fg       uintptr
	gone     map[uintptr]bool
	events   []string
	requests []uintptr
	pauses   []time.Duration
	onSet    func(uintptr) error
	onSync   func(uintptr, time.Duration) error
	onPause  func(time.Duration)
	onHold   func(time.Duration)
	onScan   func(Key, bool) error
}

func newForegroundFixture(t *testing.T) *foregroundFixture {
	t.Helper()
	preservePR1Seams(t)
	f := &foregroundFixture{t: t, now: time.Unix(0, 0), fg: fgPrevious, gone: make(map[uintptr]bool)}
	inputAncestorRoot = func(h uintptr) uintptr {
		if f.gone[h] {
			return 0
		}
		return h
	}
	inputVirtualKey = func(Key) (uintptr, bool) { return 65, true }
	inputForegroundWindow = func() uintptr { return f.fg }
	inputSetForeground = func(h uintptr) error {
		f.events = append(f.events, fmt.Sprintf("request:%d", h))
		f.requests = append(f.requests, h)
		if f.onSet != nil {
			return f.onSet(h)
		}
		f.fg = h
		return nil
	}
	foregroundNow = func() time.Time { return f.now }
	foregroundWaitSleep = func(d time.Duration) {
		if d <= 0 || d > foregroundPollInterval {
			t.Fatalf("invalid polling pause %v", d)
		}
		f.pauses = append(f.pauses, d)
		f.now = f.now.Add(d)
		if f.onPause != nil {
			f.onPause(d)
		}
	}
	foregroundHotkeySleep = func(d time.Duration) {
		f.now = f.now.Add(d)
		if f.onHold != nil {
			f.onHold(d)
		}
	}
	foregroundSync = func(h uintptr, d time.Duration) error {
		if d <= 0 || d > foregroundTransitionTimeout {
			t.Fatalf("invalid acknowledgement budget %v", d)
		}
		f.events = append(f.events, fmt.Sprintf("ack:%d", h))
		if f.onSync != nil {
			return f.onSync(h, d)
		}
		if f.gone[h] {
			return ErrWindowGone
		}
		return nil
	}
	foregroundScanCode = func(k Key, up bool) error {
		f.events = append(f.events, fmt.Sprintf("scan:%x:%t", uint16(k), up))
		if f.onScan != nil {
			return f.onScan(k, up)
		}
		return nil
	}
	return f
}

func (f *foregroundFixture) call() error {
	return pressHotkeyForeground(fgTarget, []Key{KeyCtrl, KeyA})
}
func (f *foregroundFixture) scans() []string {
	var s []string
	for _, e := range f.events {
		if strings.HasPrefix(e, "scan:") {
			s = append(s, e)
		}
	}
	return s
}
func (f *foregroundFixture) noScans() {
	f.t.Helper()
	if len(f.scans()) != 0 {
		f.t.Fatalf("unexpected input: %v", f.events)
	}
}

func TestForegroundDelayedActivationAndRestore(t *testing.T) {
	f := newForegroundFixture(t)
	f.onSet = func(uintptr) error { return nil }
	// A is initially observed; the accepted activation is not visible until
	// three fake polling ticks. Restoring A also completes asynchronously.
	f.onPause = func(time.Duration) {
		if f.now.Sub(time.Unix(0, 0)) >= 30*time.Millisecond {
			f.fg = fgTarget
		}
	}
	f.onSync = func(h uintptr, _ time.Duration) error {
		if h == fgPrevious {
			f.now = f.now.Add(20 * time.Millisecond)
			f.fg = fgPrevious
		}
		return nil
	}
	f.onScan = func(Key, bool) error {
		if f.fg != fgTarget {
			t.Fatal("sent to wrong foreground")
		}
		return nil
	}
	if err := f.call(); err != nil {
		t.Fatal(err)
	}
	if f.fg != fgPrevious || len(f.pauses) != 3 {
		t.Fatalf("foreground=%d pauses=%v", f.fg, f.pauses)
	}
	want := []string{"request:7", "ack:7", "scan:1d:false", "scan:1e:false", "scan:1e:true", "scan:1d:true", "ack:7", "request:99", "ack:99"}
	expectEvents(t, f.events, want)
	if !reflect.DeepEqual(f.requests, []uintptr{fgTarget, fgPrevious}) {
		t.Fatal(f.requests)
	}
}

func TestForegroundNeverActivatesOrReportsZero(t *testing.T) {
	for _, unconfirmed := range []uintptr{fgPrevious, 0, fgOther} {
		t.Run(fmt.Sprint(unconfirmed), func(t *testing.T) {
			f := newForegroundFixture(t)
			f.onSet = func(h uintptr) error {
				if h == fgTarget {
					f.fg = unconfirmed
				} else {
					f.fg = h
				}
				return nil
			}
			err := f.call()
			if !errors.Is(err, ErrPermissionDenied) {
				t.Fatal(err)
			}
			f.noScans()
			if len(f.pauses) != 50 || f.now.Sub(time.Unix(0, 0)) != foregroundTransitionTimeout {
				t.Fatalf("unbounded wait: %v %v", f.pauses, f.now)
			}
			if strings.Contains(err.Error(), "ASFW") {
				t.Fatal("must not diagnose the OS permission cause")
			}
		})
	}
}

func TestForegroundFailedActivationDrainsPendingBeforeRestore(t *testing.T) {
	f := newForegroundFixture(t)
	setErr := errors.New("request failed after side effect")
	pending := false
	f.onSet = func(h uintptr) error {
		if h == fgTarget {
			pending = true
			return setErr
		}
		if pending {
			t.Fatal("restored before draining target")
		}
		return nil
	}
	f.onSync = func(h uintptr, _ time.Duration) error {
		if h == fgTarget {
			if f.fg != fgPrevious {
				t.Fatal("fixture must initially still report A")
			}
			f.now = f.now.Add(30 * time.Millisecond)
			f.fg = fgTarget
			pending = false
		} else {
			f.now = f.now.Add(20 * time.Millisecond)
			f.fg = fgPrevious
		}
		return nil
	}
	err := f.call()
	if !errors.Is(err, ErrPermissionDenied) || !strings.Contains(err.Error(), setErr.Error()) {
		t.Fatal(err)
	}
	f.noScans()
	if f.fg != fgPrevious || pending {
		t.Fatal("not restored")
	}
	expectEvents(t, f.events, []string{"request:7", "ack:7", "request:99", "ack:99"})
}

func TestForegroundTimedOutTargetCanBeDrainedDuringRestore(t *testing.T) {
	f := newForegroundFixture(t)
	ackErr := errors.New("first acknowledgement timed out")
	n := 0
	f.onSet = func(h uintptr) error {
		if h == fgPrevious {
			f.fg = h
		}
		return nil
	}
	f.onSync = func(h uintptr, d time.Duration) error {
		if h == fgTarget {
			n++
			if n == 1 {
				f.now = f.now.Add(d)
				return ackErr
			}
			f.now = f.now.Add(20 * time.Millisecond)
			f.fg = fgTarget
		}
		return nil
	}
	err := f.call()
	if !errors.Is(err, ErrPermissionDenied) || !strings.Contains(err.Error(), ackErr.Error()) {
		t.Fatal(err)
	}
	if strings.Contains(err.Error(), "completion unconfirmed") {
		t.Fatal("the later acknowledgement succeeded")
	}
	f.noScans()
	if f.fg != fgPrevious || n != 2 {
		t.Fatal(f.events)
	}
}

func TestForegroundPendingTargetTimeoutIsNeverCalledRestored(t *testing.T) {
	f := newForegroundFixture(t)
	pending := false
	f.onSet = func(h uintptr) error {
		if h == fgTarget {
			pending = true
		} else {
			f.fg = h
		}
		return nil
	}
	f.onSync = func(h uintptr, d time.Duration) error {
		if h == fgTarget {
			f.now = f.now.Add(d)
			return errors.New("queue unresponsive")
		}
		return nil
	}
	err := f.call()
	if !errors.Is(err, ErrPermissionDenied) || !strings.Contains(err.Error(), "target activation completion unconfirmed") || !strings.Contains(err.Error(), "delayed foreground switch may still occur") {
		t.Fatal(err)
	}
	f.noScans()
	if f.fg != fgPrevious || !pending {
		t.Fatal("bad fixture")
	}
	// An OS timeout cannot cancel its pending activation. The operation must
	// report uncertainty even while A matches; never claim absolute recovery.
	f.fg = fgTarget
	if f.now.Sub(time.Unix(0, 0)) > 2*foregroundTransitionTimeout {
		t.Fatal("exceeded confirmation budgets")
	}
}

func TestForegroundPartialFailureCleansBeforeRestoreConfirmation(t *testing.T) {
	f := newForegroundFixture(t)
	downErr, upErr, restoreErr := errors.New("down"), errors.New("up"), errors.New("restore")
	f.onScan = func(k Key, up bool) error {
		if k == KeyA {
			return downErr
		}
		if up {
			return upErr
		}
		return nil
	}
	f.onSet = func(h uintptr) error {
		f.fg = h
		if h == fgPrevious {
			return restoreErr
		}
		return nil
	}
	err := f.call()
	if !errors.Is(err, downErr) || !errors.Is(err, upErr) || !errors.Is(err, ErrPermissionDenied) {
		t.Fatal(err)
	}
	if errors.Is(err, restoreErr) {
		t.Fatal("preserve existing restoration-error matching")
	}
	expectEvents(t, f.events, []string{"request:7", "ack:7", "scan:1d:false", "scan:1e:false", "scan:1d:true", "scan:1d:true", "ack:7", "request:99", "ack:99"})
}

func TestForegroundLostBeforeOrBetweenKeyDowns(t *testing.T) {
	for _, when := range []string{"settle", "between"} {
		t.Run(when, func(t *testing.T) {
			f := newForegroundFixture(t)
			f.onHold = func(d time.Duration) {
				if (when == "settle" && d == 50*time.Millisecond) || (when == "between" && d == 10*time.Millisecond) {
					f.fg = fgOther
				}
			}
			err := f.call()
			if !errors.Is(err, ErrPermissionDenied) {
				t.Fatal(err)
			}
			if when == "settle" {
				f.noScans()
			} else {
				expectEvents(t, f.scans(), []string{"scan:1d:false", "scan:1d:true"})
			}
			if f.fg != fgPrevious {
				t.Fatal("restore not attempted")
			}
		})
	}
}

func TestForegroundDestroyedTargetOrPrevious(t *testing.T) {
	for _, which := range []string{"target", "previous"} {
		t.Run(which, func(t *testing.T) {
			f := newForegroundFixture(t)
			f.onSet = func(h uintptr) error {
				f.fg = h
				if h == fgTarget && which == "target" {
					f.gone[fgTarget] = true
				}
				if h == fgPrevious && which == "previous" {
					f.gone[fgPrevious] = true
				}
				return nil
			}
			err := f.call()
			if !errors.Is(err, ErrPermissionDenied) || !strings.Contains(err.Error(), ErrWindowGone.Error()) {
				t.Fatal(err)
			}
			if which == "target" {
				f.noScans()
			} else if len(f.scans()) != 4 {
				t.Fatal(f.events)
			}
		})
	}
}

func TestForegroundRestoreTimeoutKeepsOriginalError(t *testing.T) {
	f := newForegroundFixture(t)
	downErr := errors.New("original down failure")
	f.onScan = func(k Key, up bool) error {
		if k == KeyA && !up {
			return downErr
		}
		return nil
	}
	f.onSet = func(h uintptr) error {
		if h == fgTarget {
			f.fg = h
		}
		return nil
	}
	err := f.call()
	if !errors.Is(err, downErr) || !errors.Is(err, ErrPermissionDenied) || !strings.Contains(err.Error(), "restored foreground was not confirmed") {
		t.Fatal(err)
	}
	expectEvents(t, f.scans(), []string{"scan:1d:false", "scan:1e:false", "scan:1d:true"})
	if f.fg != fgTarget || len(f.pauses) != 50 {
		t.Fatal(f.events, f.pauses)
	}
}

func TestForegroundZeroPreviousAndAlreadyForeground(t *testing.T) {
	t.Run("zero previous refuses before any request", func(t *testing.T) {
		f := newForegroundFixture(t)
		f.fg = 0
		if err := f.call(); !errors.Is(err, ErrPermissionDenied) {
			t.Fatal(err)
		}
		f.noScans()
		if len(f.events) != 0 {
			t.Fatal(f.events)
		}
	})
	t.Run("already foreground does not switch", func(t *testing.T) {
		f := newForegroundFixture(t)
		f.fg = fgTarget
		if err := f.call(); err != nil {
			t.Fatal(err)
		}
		expectEvents(t, f.events, []string{"scan:1d:false", "scan:1e:false", "scan:1e:true", "scan:1d:true"})
	})
}

func TestForegroundAcknowledgementConsumesSharedDeadline(t *testing.T) {
	f := newForegroundFixture(t)
	f.fg = fgTarget
	f.onSync = func(_ uintptr, d time.Duration) error { f.now = f.now.Add(d); return nil }
	if err := confirmHotkeyForeground(fgTarget, f.now.Add(foregroundTransitionTimeout)); err == nil || len(f.pauses) != 0 {
		t.Fatal("acknowledgement exhausted the entire budget", err, f.pauses)
	}
	f.fg = fgPrevious
	f.onSync = func(_ uintptr, _ time.Duration) error { f.now = f.now.Add(3 * time.Millisecond); return nil }
	f.pauses = nil
	if err := confirmHotkeyForeground(fgTarget, f.now.Add(25*time.Millisecond)); err == nil {
		t.Fatal("never became foreground")
	}
	if !reflect.DeepEqual(f.pauses, []time.Duration{10 * time.Millisecond, 10 * time.Millisecond, 2 * time.Millisecond}) {
		t.Fatal(f.pauses)
	}
}

func TestForegroundRestoreRequiresAcknowledgementEvenWhenAlreadyA(t *testing.T) {
	f := newForegroundFixture(t)
	f.onSync = func(h uintptr, _ time.Duration) error {
		if h == fgPrevious {
			return errors.New("restore acknowledgement failed")
		}
		return nil
	}
	err := f.call()
	if !errors.Is(err, ErrPermissionDenied) || !strings.Contains(err.Error(), "restored foreground was not confirmed") {
		t.Fatal(err)
	}
	if f.fg != fgPrevious || len(f.scans()) != 4 {
		t.Fatal(f.events)
	}
}

func TestForegroundPreflightStillWinsBeforeTransition(t *testing.T) {
	f := newForegroundFixture(t)
	f.fg = 0
	inputVirtualKey = func(Key) (uintptr, bool) { return 0, false }
	if err := f.call(); !errors.Is(err, ErrUnsupportedKey) {
		t.Fatal(err)
	}
	if len(f.events) != 0 {
		t.Fatal(f.events)
	}
}
