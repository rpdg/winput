package winput_test

import (
	"errors"
	"testing"

	"github.com/rpdg/winput"
)

func TestPublicProbeAndInput(t *testing.T) {
	if err := winput.SetBackend(winput.BackendMessage); err != nil {
		t.Fatal(err)
	}

	msg, err := winput.NewInput(winput.BackendMessage)
	if err != nil || msg == nil {
		t.Fatalf("message executor = %v, %v", msg, err)
	}
	illegal, err := winput.NewInput(winput.Backend(99))
	if illegal != nil || !errors.Is(err, winput.ErrInvalidBackend) {
		t.Fatalf("illegal executor = %v, %v", illegal, err)
	}
	if err.Error() != "invalid input backend" {
		t.Fatalf("invalid backend text = %q", err.Error())
	}

	report, err := winput.ProbeTarget(nil)
	if report != nil || !errors.Is(err, winput.ErrWindowGone) {
		t.Fatalf("nil probe = %v, %v", report, err)
	}

	before := winput.ProbeHID()
	if !knownHIDStatus(before) {
		t.Fatalf("HID status %q", before)
	}
	after := winput.ProbeHID()
	if after != before {
		t.Fatalf("HID status changed from %q to %q", before, after)
	}
	// ProbeHID must not switch the process onto HID. A message move to the
	// current point stays on SetCursorPos; a switched HID backend would take
	// the driver path instead.
	x, y, err := winput.GetCursorPos()
	if err != nil {
		t.Fatal(err)
	}
	if err := winput.MoveMouseTo(x, y); err != nil {
		t.Fatalf("message move after probe = %v", err)
	}

	facade, err := msg.ForWindow(nil)
	if err != nil || facade == nil {
		t.Fatalf("ForWindow(nil) = %v, %v", facade, err)
	}
	if err := facade.Scroll(0, 0, 120); !errors.Is(err, winput.ErrWindowGone) {
		t.Fatalf("nil window scroll = %v", err)
	}
	desk, err := msg.Desktop()
	if err != nil || desk == nil {
		t.Fatal(err)
	}
	if err := desk.ScrollWheel(0); !errors.Is(err, winput.ErrInvalidScrollSteps) {
		t.Fatalf("desktop scroll = %v", err)
	}

	if err := winput.SetBackend(winput.Backend(42)); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = winput.SetBackend(winput.BackendMessage) }()
	if err := winput.ScrollWheel(0); !errors.Is(err, winput.ErrInvalidScrollSteps) {
		t.Fatalf("non-HID backend scroll = %v", err)
	}
}

func knownHIDStatus(s winput.HIDStatus) bool {
	switch s {
	case winput.HIDInitialized, winput.HIDDLLUnavailable, winput.HIDDriverUnavailable, winput.HIDDevicesUnavailable, winput.HIDUnknown:
		return s != ""
	default:
		return false
	}
}
