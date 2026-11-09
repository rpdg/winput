package winput

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/rpdg/winput/hid"
)

func TestDeriveWarnings(t *testing.T) {
	cases := []struct {
		rel    IntegrityRelation
		access UIAccess
		want   []ProbeWarning
	}{
		{IntegrityTargetHigher, UIAccessEnabled, []ProbeWarning{WarnUIPILikelyBlocked}},
		{IntegrityTargetHigher, UIAccessDisabled, []ProbeWarning{WarnUIPILikelyBlocked}},
		{IntegrityTargetHigher, UIAccessUnknown, []ProbeWarning{WarnUIPILikelyBlocked, WarnUIAccessUnknown}},
		{IntegrityTargetHigher, "", []ProbeWarning{WarnUIPILikelyBlocked, WarnUIAccessUnknown}},
		{IntegrityEqual, UIAccessEnabled, nil},
		{IntegrityEqual, UIAccessUnknown, []ProbeWarning{WarnUIAccessUnknown}},
		{IntegrityTargetLower, UIAccessDisabled, nil},
		{IntegrityUnknown, UIAccessEnabled, []ProbeWarning{WarnIntegrityUnknown}},
		{IntegrityUnknown, UIAccessUnknown, []ProbeWarning{WarnIntegrityUnknown, WarnUIAccessUnknown}},
		{"", UIAccessEnabled, []ProbeWarning{WarnIntegrityUnknown}},
	}
	for _, tc := range cases {
		got := deriveWarnings(tc.rel, tc.access)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("deriveWarnings(%q, %q) = %#v, want %#v", tc.rel, tc.access, got, tc.want)
		}
	}
}

func TestClassifyHIDUsesSentinelsNotText(t *testing.T) {
	lib := fmt.Errorf("load: %w", hid.ErrLibraryNotFound)
	cases := []struct {
		err  error
		want HIDStatus
	}{
		{nil, HIDInitialized},
		{hid.ErrDriverNotInstalled, HIDDriverUnavailable},
		{fmt.Errorf("ctx: %w", hid.ErrDriverNotInstalled), HIDDriverUnavailable},
		{hid.ErrDevicesUnavailable, HIDDevicesUnavailable},
		{hid.ErrLibraryNotFound, HIDDLLUnavailable},
		{hid.ErrSymbolsMissing, HIDDLLUnavailable},
		{lib, HIDDLLUnavailable},
		{fmt.Errorf("no interception devices found"), HIDUnknown},
		{fmt.Errorf("library loaded but symbols missing"), HIDUnknown},
		{errors.New("something else"), HIDUnknown},
	}
	for _, tc := range cases {
		got := classifyHID(tc.err)
		if got != tc.want {
			t.Errorf("classifyHID(%v) = %q, want %q", tc.err, got, tc.want)
		}
		if got == "" {
			t.Errorf("classifyHID(%v) returned an empty status", tc.err)
		}
	}
	if HIDStatus("") == HIDInitialized {
		t.Fatal("zero HIDStatus must not mean initialized")
	}
	if IntegrityRelation("") == IntegrityEqual || UIAccess("") == UIAccessEnabled {
		t.Fatal("zero fact values must not mean a confirmed fact")
	}
}

func TestTranslateHIDInitKeepsPublicErrors(t *testing.T) {
	driver := translateHIDInit(hid.ErrDriverNotInstalled)
	if !errors.Is(driver, ErrDriverNotInstalled) || errors.Is(driver, ErrDLLLoadFailed) {
		t.Fatalf("driver error = %v", driver)
	}
	for _, err := range []error{hid.ErrDevicesUnavailable, hid.ErrSymbolsMissing, hid.ErrLibraryNotFound, errors.New("other")} {
		got := translateHIDInit(err)
		if !errors.Is(got, ErrDLLLoadFailed) {
			t.Fatalf("translate(%v) = %v, want ErrDLLLoadFailed", err, got)
		}
		if errors.Is(got, ErrDriverNotInstalled) || errors.Is(got, hid.ErrDevicesUnavailable) || errors.Is(got, hid.ErrSymbolsMissing) {
			t.Fatalf("translate(%v) exposed a hid sentinel: %v", err, got)
		}
	}
}

func TestResolveIdentity(t *testing.T) {
	base := idSnap{valid: true, idOK: true, pid: 7, tid: 9, class: classSnap{known: true, value: "Edit"}}
	cases := []struct {
		name  string
		start idSnap
		end   idSnap
		class string
		err   error
	}{
		{"invalid start", idSnap{}, base, "", ErrWindowGone},
		{"invalid end", base, idSnap{idOK: true}, "", ErrWindowGone},
		{"start pid missing", idSnap{valid: true}, base, "", ErrWindowGone},
		{"end tid missing", base, idSnap{valid: true, pid: 7}, "", ErrWindowGone},
		{"start pid zero", idSnap{valid: true, idOK: true, pid: 0, tid: 9}, base, "", ErrWindowGone},
		{"start tid zero", idSnap{valid: true, idOK: true, pid: 7, tid: 0}, base, "", ErrWindowGone},
		{"end pid zero", base, idSnap{valid: true, idOK: true, pid: 0, tid: 9}, "", ErrWindowGone},
		{"end tid zero", base, idSnap{valid: true, idOK: true, pid: 7, tid: 0}, "", ErrWindowGone},
		{"pid changed", base, idSnap{valid: true, idOK: true, pid: 8, tid: 9, class: base.class}, "", ErrTargetChanged},
		{"tid changed", base, idSnap{valid: true, idOK: true, pid: 7, tid: 10, class: base.class}, "", ErrTargetChanged},
		{"class changed", base, idSnap{valid: true, idOK: true, pid: 7, tid: 9, class: classSnap{known: true, value: "Button"}}, "", ErrTargetChanged},
		{"end class unread keeps start", base, idSnap{valid: true, idOK: true, pid: 7, tid: 9}, "Edit", nil},
		{"start unread uses end", idSnap{valid: true, idOK: true, pid: 7, tid: 9}, base, "Edit", nil},
		{"both unread", idSnap{valid: true, idOK: true, pid: 7, tid: 9}, idSnap{valid: true, idOK: true, pid: 7, tid: 9}, "", nil},
		{"empty class is a value", idSnap{valid: true, idOK: true, pid: 1, tid: 2, class: classSnap{known: true, value: ""}}, idSnap{valid: true, idOK: true, pid: 1, tid: 2, class: classSnap{known: true, value: ""}}, "", nil},
		{"empty then different", idSnap{valid: true, idOK: true, pid: 1, tid: 2, class: classSnap{known: true, value: ""}}, idSnap{valid: true, idOK: true, pid: 1, tid: 2, class: classSnap{known: true, value: "Edit"}}, "", ErrTargetChanged},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveIdentity(tc.start, tc.end)
			if !errors.Is(err, tc.err) || got != tc.class {
				t.Fatalf("resolve = %q, %v; want class %q err %v", got, err, tc.class, tc.err)
			}
		})
	}
}

func TestProbeTargetNilAndSeams(t *testing.T) {
	inits := 0
	restoreInit := swapInit(func() error {
		inits++
		return errors.New("probe must not init")
	})
	defer restoreInit()

	report, err := ProbeTarget(nil)
	if report != nil || !errors.Is(err, ErrWindowGone) {
		t.Fatalf("nil window = %v, %v", report, err)
	}
	report, err = ProbeTarget(&Window{})
	if report != nil || !errors.Is(err, ErrWindowGone) {
		t.Fatalf("zero hwnd = %v, %v", report, err)
	}

	const hwnd = uintptr(1)
	stable := idSnap{valid: true, idOK: true, pid: 4, tid: 5}
	restoreID := swapIdentity(func(got uintptr) idSnap {
		if got != hwnd {
			t.Fatalf("identity read used hwnd %d", got)
		}
		return stable
	})
	defer restoreID()
	handle := syscall.Handle(0x77)
	opens := 0
	restoreOpen := swapOpenProcess(func(pid uint32) (syscall.Handle, bool) {
		opens++
		if pid != stable.pid {
			t.Fatalf("opened pid %d, want %d", pid, stable.pid)
		}
		return handle, true
	})
	defer restoreOpen()
	closes := 0
	restoreClose := swapCloseProcess(func(got syscall.Handle) {
		closes++
		if got != handle {
			t.Fatalf("closed handle %v, want %v", got, handle)
		}
	})
	defer restoreClose()
	restoreFacts := swapFacts(func(proc syscall.Handle, openOK bool) (string, IntegrityRelation, UIAccess) {
		if !openOK || proc != handle {
			t.Fatalf("facts handle = %v, openOK = %v", proc, openOK)
		}
		return "", IntegrityUnknown, UIAccessUnknown
	})
	defer restoreFacts()

	report, err = ProbeTarget(&Window{HWND: hwnd})
	if err != nil || report == nil {
		t.Fatalf("unknown facts = %v, %v", report, err)
	}
	if report.Features.TargetClass != "" || report.Features.ProcessName != "" {
		t.Fatalf("facts = %+v", report.Features)
	}
	if report.Features.IntegrityRelation != IntegrityUnknown || report.Features.CallerUIAccess != UIAccessUnknown {
		t.Fatalf("tokens = %+v", report.Features)
	}
	want := []ProbeWarning{WarnIntegrityUnknown, WarnUIAccessUnknown}
	if !reflect.DeepEqual(report.Warnings, want) {
		t.Fatalf("warnings = %#v", report.Warnings)
	}
	if opens != 1 || closes != 1 {
		t.Fatalf("opens/closes = %d/%d, want 1/1", opens, closes)
	}
	if inits != 0 {
		t.Fatalf("ProbeTarget called HID init %d times", inits)
	}
	assertNoSuggestionFields(t)
}

func TestProbeTargetStartRejectsWithoutOpening(t *testing.T) {
	cases := []struct {
		name string
		snap idSnap
	}{
		{"invalid hwnd", idSnap{}},
		{"identity unreadable", idSnap{valid: true}},
		{"pid zero", idSnap{valid: true, idOK: true, pid: 0, tid: 9}},
		{"tid zero", idSnap{valid: true, idOK: true, pid: 7, tid: 0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			restoreID := swapIdentity(func(uintptr) idSnap { return tc.snap })
			defer restoreID()
			opens := 0
			restoreOpen := swapOpenProcess(func(uint32) (syscall.Handle, bool) {
				opens++
				return 1, true
			})
			defer restoreOpen()

			report, err := ProbeTarget(&Window{HWND: 1})
			if report != nil || !errors.Is(err, ErrWindowGone) {
				t.Fatalf("probe = %v, %v; want ErrWindowGone", report, err)
			}
			if opens != 0 {
				t.Fatalf("opened process %d times for an unusable start snapshot", opens)
			}
		})
	}
}

func TestProbeTargetDoesNotTakeInputMutexOrChangeBackend(t *testing.T) {
	prev := getBackend()
	defer setGlobalBackend(prev)
	setGlobalBackend(BackendHID)

	const hwnd = uintptr(6)
	stable := idSnap{valid: true, idOK: true, pid: 4, tid: 5}
	restoreID := swapIdentity(func(uintptr) idSnap { return stable })
	defer restoreID()
	restoreOpen := swapOpenProcess(func(uint32) (syscall.Handle, bool) { return 0, false })
	defer restoreOpen()
	restoreFacts := swapFacts(func(syscall.Handle, bool) (string, IntegrityRelation, UIAccess) {
		return "", IntegrityUnknown, UIAccessUnknown
	})
	defer restoreFacts()

	inputMutex.Lock()
	done := make(chan error, 1)
	go func() {
		_, err := ProbeTarget(&Window{HWND: hwnd})
		done <- err
	}()
	select {
	case err := <-done:
		inputMutex.Unlock()
		if err != nil {
			t.Fatalf("probe under held inputMutex = %v", err)
		}
	case <-time.After(2 * time.Second):
		inputMutex.Unlock()
		t.Fatal("ProbeTarget waited for inputMutex")
	}
	if got := getBackend(); got != BackendHID {
		t.Fatalf("ProbeTarget changed the global backend to %v", got)
	}
}

func TestProbeTargetHoldsOneProcessHandle(t *testing.T) {
	const hwnd = uintptr(11)
	stable := idSnap{valid: true, idOK: true, pid: 77, tid: 8, class: classSnap{known: true, value: "Edit"}}
	w := &Window{HWND: hwnd}
	var events []string
	restoreID := swapIdentity(func(got uintptr) idSnap {
		if got != hwnd {
			t.Fatalf("identity read used hwnd %d, want %d", got, hwnd)
		}
		events = append(events, "identity")
		return stable
	})
	defer restoreID()
	handle := syscall.Handle(0x1234)
	opens := 0
	restoreOpen := swapOpenProcess(func(pid uint32) (syscall.Handle, bool) {
		opens++
		if pid != stable.pid {
			t.Fatalf("opened pid %d, want %d", pid, stable.pid)
		}
		events = append(events, "open")
		return handle, true
	})
	defer restoreOpen()
	closes := 0
	restoreClose := swapCloseProcess(func(got syscall.Handle) {
		closes++
		if got != handle {
			t.Fatalf("closed handle %v, want %v", got, handle)
		}
		events = append(events, "close")
	})
	defer restoreClose()
	restoreFacts := swapFacts(func(proc syscall.Handle, openOK bool) (string, IntegrityRelation, UIAccess) {
		if !openOK || proc != handle {
			t.Fatalf("facts handle = %v, openOK = %v; want %v/true", proc, openOK, handle)
		}
		events = append(events, "facts")
		w.HWND = 999 // The probe must keep the hwnd read at entry.
		return "notepad.exe", IntegrityEqual, UIAccessEnabled
	})
	defer restoreFacts()

	report, err := ProbeTarget(w)
	if err != nil || report == nil {
		t.Fatalf("probe = %v, %v", report, err)
	}
	wantEvents := []string{"identity", "open", "identity", "facts", "identity", "close"}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("events = %#v, want %#v", events, wantEvents)
	}
	if opens != 1 || closes != 1 {
		t.Fatalf("opens/closes = %d/%d, want 1/1", opens, closes)
	}
	if report.Features.ProcessName != "notepad.exe" ||
		report.Features.TargetClass != "Edit" ||
		report.Features.IntegrityRelation != IntegrityEqual ||
		report.Features.CallerUIAccess != UIAccessEnabled {
		t.Fatalf("features = %+v", report.Features)
	}
}

func TestProbeTargetAfterOpenResolution(t *testing.T) {
	const hwnd = uintptr(21)
	known := idSnap{valid: true, idOK: true, pid: 4, tid: 5, class: classSnap{known: true, value: "Edit"}}
	withoutID := idSnap{valid: true, idOK: true, pid: 4, tid: 5}
	unknownClass := idSnap{valid: true, idOK: true, pid: 4, tid: 5}
	emptyKnown := idSnap{valid: true, idOK: true, pid: 4, tid: 5, class: classSnap{known: true, value: ""}}

	cases := []struct {
		name      string
		snaps     []idSnap
		wantErr   error
		wantClass string
		wantFacts int
	}{
		{"gone after open", []idSnap{known, {}}, ErrWindowGone, "", 0},
		{"identity unreadable after open", []idSnap{known, {valid: true}}, ErrWindowGone, "", 0},
		{"pid zero after open", []idSnap{known, {valid: true, idOK: true, pid: 0, tid: 5, class: known.class}}, ErrWindowGone, "", 0},
		{"tid zero after open", []idSnap{known, {valid: true, idOK: true, pid: 4, tid: 0, class: known.class}}, ErrWindowGone, "", 0},
		{"pid changed after open", []idSnap{known, {valid: true, idOK: true, pid: 9, tid: 5, class: known.class}}, ErrTargetChanged, "", 0},
		{"tid changed after open", []idSnap{known, {valid: true, idOK: true, pid: 4, tid: 9, class: known.class}}, ErrTargetChanged, "", 0},
		{"class changed after open", []idSnap{known, {valid: true, idOK: true, pid: 4, tid: 5, class: classSnap{known: true, value: "Button"}}}, ErrTargetChanged, "", 0},
		{"class unread after open keeps start", []idSnap{known, withoutID, known}, nil, "Edit", 1},
		{"empty known class changed after open", []idSnap{emptyKnown, {valid: true, idOK: true, pid: 4, tid: 5, class: classSnap{known: true, value: "Edit"}}}, ErrTargetChanged, "", 0},
		{"start unknown is not promoted from afterOpen", []idSnap{
			unknownClass,
			{valid: true, idOK: true, pid: 4, tid: 5, class: classSnap{known: true, value: "Edit"}},
			{valid: true, idOK: true, pid: 4, tid: 5, class: classSnap{known: true, value: "Button"}},
		}, nil, "Button", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			restoreID := swapIdentity(func(got uintptr) idSnap {
				if got != hwnd {
					t.Fatalf("identity read used hwnd %d", got)
				}
				if calls >= len(tc.snaps) {
					t.Fatalf("identity read %d exceeded %d snapshots", calls+1, len(tc.snaps))
				}
				snap := tc.snaps[calls]
				calls++
				return snap
			})
			defer restoreID()
			handle := syscall.Handle(0x88)
			restoreOpen := swapOpenProcess(func(uint32) (syscall.Handle, bool) { return handle, true })
			defer restoreOpen()
			closes := 0
			restoreClose := swapCloseProcess(func(got syscall.Handle) {
				closes++
				if got != handle {
					t.Fatalf("closed handle %v", got)
				}
			})
			defer restoreClose()
			facts := 0
			restoreFacts := swapFacts(func(proc syscall.Handle, openOK bool) (string, IntegrityRelation, UIAccess) {
				facts++
				if !openOK || proc != handle {
					t.Fatalf("facts handle = %v, openOK = %v", proc, openOK)
				}
				return "notepad.exe", IntegrityEqual, UIAccessEnabled
			})
			defer restoreFacts()

			report, err := ProbeTarget(&Window{HWND: hwnd})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("probe error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil && report != nil {
				t.Fatalf("partial report on error: %+v", report)
			}
			if tc.wantErr == nil && report == nil {
				t.Fatal("probe returned no report")
			}
			if tc.wantErr == nil && report.Features.TargetClass != tc.wantClass {
				t.Fatalf("class = %q, want %q", report.Features.TargetClass, tc.wantClass)
			}
			if facts != tc.wantFacts {
				t.Fatalf("facts calls = %d, want %d", facts, tc.wantFacts)
			}
			if closes != 1 {
				t.Fatalf("closes = %d, want 1", closes)
			}
			if calls != len(tc.snaps) {
				t.Fatalf("identity calls = %d, want %d", calls, len(tc.snaps))
			}
		})
	}
}

func TestProbeTargetEndResolution(t *testing.T) {
	const hwnd = uintptr(31)
	known := idSnap{valid: true, idOK: true, pid: 4, tid: 5, class: classSnap{known: true, value: "Edit"}}
	withoutID := idSnap{valid: true, idOK: true, pid: 4, tid: 5}
	unknownClass := idSnap{valid: true, idOK: true, pid: 4, tid: 5}
	emptyKnown := idSnap{valid: true, idOK: true, pid: 4, tid: 5, class: classSnap{known: true, value: ""}}
	endButton := idSnap{valid: true, idOK: true, pid: 4, tid: 5, class: classSnap{known: true, value: "Button"}}

	cases := []struct {
		name      string
		start     idSnap
		end       idSnap
		wantErr   error
		wantClass string
	}{
		{"gone at end", known, idSnap{}, ErrWindowGone, ""},
		{"identity unreadable at end", known, idSnap{valid: true}, ErrWindowGone, ""},
		{"pid changed at end", known, idSnap{valid: true, idOK: true, pid: 9, tid: 5, class: known.class}, ErrTargetChanged, ""},
		{"tid changed at end", known, idSnap{valid: true, idOK: true, pid: 4, tid: 9, class: known.class}, ErrTargetChanged, ""},
		{"class changed at end", known, endButton, ErrTargetChanged, ""},
		{"empty known class changed at end", emptyKnown, endButton, ErrTargetChanged, ""},
		{"end class unread uses start", known, withoutID, nil, "Edit"},
		{"start unknown end class readable", unknownClass, known, nil, "Edit"},
		{"both class unknown", unknownClass, unknownClass, nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			restoreID := swapIdentity(func(got uintptr) idSnap {
				if got != hwnd {
					t.Fatalf("identity read used hwnd %d", got)
				}
				calls++
				if calls == 3 {
					return tc.end
				}
				return tc.start
			})
			defer restoreID()
			handle := syscall.Handle(0x99)
			restoreOpen := swapOpenProcess(func(uint32) (syscall.Handle, bool) { return handle, true })
			defer restoreOpen()
			closes := 0
			restoreClose := swapCloseProcess(func(got syscall.Handle) {
				closes++
				if got != handle {
					t.Fatalf("closed handle %v", got)
				}
			})
			defer restoreClose()
			facts := 0
			restoreFacts := swapFacts(func(proc syscall.Handle, openOK bool) (string, IntegrityRelation, UIAccess) {
				facts++
				return "notepad.exe", IntegrityEqual, UIAccessEnabled
			})
			defer restoreFacts()

			report, err := ProbeTarget(&Window{HWND: hwnd})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("probe error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil && report != nil {
				t.Fatalf("partial report on error: %+v", report)
			}
			if tc.wantErr == nil {
				if report == nil {
					t.Fatal("probe returned no report")
				}
				if report.Features.TargetClass != tc.wantClass {
					t.Fatalf("class = %q, want %q", report.Features.TargetClass, tc.wantClass)
				}
				if report.Features.ProcessName != "notepad.exe" {
					t.Fatalf("name = %q", report.Features.ProcessName)
				}
			}
			if facts != 1 {
				t.Fatalf("facts calls = %d, want 1", facts)
			}
			if closes != 1 {
				t.Fatalf("closes = %d, want 1", closes)
			}
			if calls != 3 {
				t.Fatalf("identity calls = %d, want 3", calls)
			}
		})
	}
}

func TestProbeTargetOpenFailureDegradesWithoutReopen(t *testing.T) {
	const hwnd = uintptr(41)
	stable := idSnap{valid: true, idOK: true, pid: 4, tid: 5, class: classSnap{known: true, value: "Edit"}}
	reads := 0
	restoreID := swapIdentity(func(got uintptr) idSnap {
		if got != hwnd {
			t.Fatalf("identity read used hwnd %d", got)
		}
		reads++
		return stable
	})
	defer restoreID()
	opens := 0
	restoreOpen := swapOpenProcess(func(pid uint32) (syscall.Handle, bool) {
		opens++
		if pid != stable.pid {
			t.Fatalf("opened pid %d", pid)
		}
		return 0, false
	})
	defer restoreOpen()
	closes := 0
	restoreClose := swapCloseProcess(func(syscall.Handle) { closes++ })
	defer restoreClose()
	inits := 0
	restoreInit := swapInit(func() error {
		inits++
		return errors.New("probe must not init")
	})
	defer restoreInit()
	facts := 0
	restoreFacts := swapFacts(func(proc syscall.Handle, openOK bool) (string, IntegrityRelation, UIAccess) {
		facts++
		if proc != 0 || openOK {
			t.Fatalf("facts handle = %v, openOK = %v; want degraded", proc, openOK)
		}
		return "", IntegrityUnknown, UIAccessEnabled
	})
	defer restoreFacts()

	report, err := ProbeTarget(&Window{HWND: hwnd})
	if err != nil || report == nil {
		t.Fatalf("probe = %v, %v", report, err)
	}
	if report.Features.ProcessName != "" || report.Features.IntegrityRelation != IntegrityUnknown {
		t.Fatalf("features = %+v", report.Features)
	}
	if report.Features.CallerUIAccess != UIAccessEnabled {
		t.Fatalf("UIAccess = %q, want enabled", report.Features.CallerUIAccess)
	}
	want := []ProbeWarning{WarnIntegrityUnknown}
	if !reflect.DeepEqual(report.Warnings, want) {
		t.Fatalf("warnings = %#v", report.Warnings)
	}
	if opens != 1 || closes != 0 || facts != 1 || reads != 2 {
		t.Fatalf("opens/closes/facts/reads = %d/%d/%d/%d, want 1/0/1/2", opens, closes, facts, reads)
	}
	if inits != 0 {
		t.Fatalf("open failure called HID init %d times", inits)
	}
}

func TestReadProcessFactsDegradesIndependently(t *testing.T) {
	t.Run("open failure reads neither name nor token", func(t *testing.T) {
		nameReads, tokenOpens := 0, 0
		restoreName := swapTargetName(func(syscall.Handle) string { nameReads++; return "x" })
		defer restoreName()
		restoreToken := swapTargetToken(func(uintptr) (syscall.Handle, bool) { tokenOpens++; return 1, true })
		defer restoreToken()

		name, rid, ok := readProcessFacts(123, false)
		if name != "" || rid != 0 || ok {
			t.Fatalf("facts = %q/%d/%v, want empty/0/false", name, rid, ok)
		}
		if nameReads != 0 || tokenOpens != 0 {
			t.Fatalf("name/token reads = %d/%d, want 0/0", nameReads, tokenOpens)
		}
	})

	t.Run("token failure retains the name", func(t *testing.T) {
		restoreName := swapTargetName(func(syscall.Handle) string { return "notepad.exe" })
		defer restoreName()
		restoreToken := swapTargetToken(func(uintptr) (syscall.Handle, bool) { return 0, false })
		defer restoreToken()
		closed := 0
		restoreClose := swapTargetTokenClose(func(syscall.Handle) { closed++ })
		defer restoreClose()

		name, rid, ok := readProcessFacts(123, true)
		if name != "notepad.exe" || rid != 0 || ok {
			t.Fatalf("facts = %q/%d/%v", name, rid, ok)
		}
		if closed != 0 {
			t.Fatalf("closed %d unopened token handles", closed)
		}
	})

	t.Run("name failure retains token facts", func(t *testing.T) {
		token := syscall.Handle(55)
		restoreName := swapTargetName(func(syscall.Handle) string { return "" })
		defer restoreName()
		restoreToken := swapTargetToken(func(uintptr) (syscall.Handle, bool) { return token, true })
		defer restoreToken()
		restoreRID := swapTargetIntegrity(func(got syscall.Handle) (uint32, bool) {
			if got != token {
				t.Fatalf("integrity used handle %v", got)
			}
			return 0x3000, true
		})
		defer restoreRID()
		closed := 0
		restoreClose := swapTargetTokenClose(func(got syscall.Handle) {
			closed++
			if got != token {
				t.Fatalf("closed token %v", got)
			}
		})
		defer restoreClose()

		name, rid, ok := readProcessFacts(123, true)
		if name != "" || rid != 0x3000 || !ok {
			t.Fatalf("facts = %q/%#x/%v", name, rid, ok)
		}
		if closed != 1 {
			t.Fatalf("token closes = %d, want 1", closed)
		}
	})

	t.Run("integrity failure retains the name and closes the token", func(t *testing.T) {
		token := syscall.Handle(56)
		restoreName := swapTargetName(func(syscall.Handle) string { return "notepad.exe" })
		defer restoreName()
		restoreToken := swapTargetToken(func(uintptr) (syscall.Handle, bool) { return token, true })
		defer restoreToken()
		restoreRID := swapTargetIntegrity(func(syscall.Handle) (uint32, bool) { return 0, false })
		defer restoreRID()
		closed := 0
		restoreClose := swapTargetTokenClose(func(got syscall.Handle) {
			closed++
			if got != token {
				t.Fatalf("closed token %v", got)
			}
		})
		defer restoreClose()

		name, rid, ok := readProcessFacts(123, true)
		if name != "notepad.exe" || rid != 0 || ok {
			t.Fatalf("facts = %q/%d/%v", name, rid, ok)
		}
		if closed != 1 {
			t.Fatalf("token closes = %d, want 1", closed)
		}
	})
}

// TestReadProcessFactsUsesSameProcessHandle verifies the lower half of the
// handle chain; TestProbeTargetHoldsOneProcessHandle verifies the probe passes
// its opened handle into collectFacts.
func TestReadProcessFactsUsesSameProcessHandle(t *testing.T) {
	const proc = syscall.Handle(0x1234)
	var nameHandle, tokenHandle syscall.Handle
	restoreName := swapTargetName(func(got syscall.Handle) string {
		nameHandle = got
		return "notepad.exe"
	})
	defer restoreName()
	restoreToken := swapTargetToken(func(got uintptr) (syscall.Handle, bool) {
		tokenHandle = syscall.Handle(got)
		return 0, false
	})
	defer restoreToken()

	name, rid, ok := readProcessFacts(proc, true)
	if name != "notepad.exe" || rid != 0 || ok {
		t.Fatalf("facts = %q/%d/%v", name, rid, ok)
	}
	if nameHandle != proc {
		t.Fatalf("name read used handle %v, want %v", nameHandle, proc)
	}
	if tokenHandle != proc {
		t.Fatalf("token read used handle %v, want %v", tokenHandle, proc)
	}
}

func TestNewInputAndTaskFacade(t *testing.T) {
	prev := getBackend()
	defer func() { _ = SetBackend(prev) }()

	inits := 0
	restore := swapInit(func() error {
		inits++
		return errors.New("unexpected init")
	})
	defer restore()

	msg, err := NewInput(BackendMessage)
	if err != nil || msg == nil || !msg.valid || msg.backend != BackendMessage {
		t.Fatalf("message input = %+v, %v", msg, err)
	}
	if inits != 0 {
		t.Fatal("BackendMessage initialized HID")
	}
	for _, bad := range []Backend{2, Backend(-1), Backend(99)} {
		got, err := NewInput(bad)
		if got != nil || !errors.Is(err, ErrInvalidBackend) {
			t.Fatalf("NewInput(%d) = %v, %v", bad, got, err)
		}
	}
	if inits != 0 {
		t.Fatal("illegal backend initialized HID")
	}
	if ErrInvalidBackend.Error() != "invalid input backend" {
		t.Fatalf("error text = %q", ErrInvalidBackend.Error())
	}

	var zero Input
	if facade, err := zero.ForWindow(&Window{}); facade != nil || !errors.Is(err, ErrInputInvalid) {
		t.Fatalf("zero ForWindow = %v, %v", facade, err)
	}
	if desk, err := zero.Desktop(); desk != nil || !errors.Is(err, ErrInputInvalid) {
		t.Fatalf("zero Desktop = %v, %v", desk, err)
	}
	var nilInput *Input
	if _, err := nilInput.ForWindow(nil); !errors.Is(err, ErrInputInvalid) {
		t.Fatalf("nil ForWindow = %v", err)
	}
	var win WindowInput
	if err := win.Click(0, 0); !errors.Is(err, ErrInputInvalid) {
		t.Fatalf("zero window facade = %v", err)
	}
	var desk DesktopInput
	if err := desk.MoveMouseTo(0, 0); !errors.Is(err, ErrInputInvalid) {
		t.Fatalf("zero desktop facade = %v", err)
	}
	var nilWin *WindowInput
	if err := nilWin.Scroll(0, 0, 120); !errors.Is(err, ErrInputInvalid) {
		t.Fatalf("nil window facade = %v", err)
	}
	var nilDesk *DesktopInput
	if err := nilDesk.KeyDown(KeyA); !errors.Is(err, ErrInputInvalid) {
		t.Fatalf("nil desktop facade = %v", err)
	}

	facade, err := msg.ForWindow(nil)
	if err != nil || facade == nil {
		t.Fatalf("ForWindow(nil) = %v, %v", facade, err)
	}
	if err := facade.Click(1, 1); !errors.Is(err, ErrWindowGone) {
		t.Fatalf("nil window click = %v", err)
	}

	if err := SetBackend(BackendHID); err == nil {
		t.Fatal("SetBackend(HID) succeeded while init is stubbed")
	}
	inits = 0
	_ = SetBackend(Backend(42))
	if getBackend() != 42 {
		t.Fatalf("backend = %v", getBackend())
	}
	// The message executor must ignore the global backend, including HID.
	backendMutex.Lock()
	currentBackend = BackendHID
	backendMutex.Unlock()
	isolated, err := msg.Desktop()
	if err != nil {
		t.Fatal(err)
	}
	err = isolated.ScrollWheel(0)
	if !errors.Is(err, ErrInvalidScrollSteps) {
		t.Fatalf("isolated scroll = %v", err)
	}
	if inits != 0 {
		t.Fatal("task input read the global HID backend and initialized")
	}
	if msg.backend != BackendMessage {
		t.Fatal("SetBackend changed an existing executor")
	}
}

func TestNonHIDSetBackendStaysOnMessagePath(t *testing.T) {
	prev := getBackend()
	defer func() { _ = SetBackend(prev) }()
	if err := SetBackend(Backend(42)); err != nil {
		t.Fatal(err)
	}
	if getBackend() != 42 {
		t.Fatalf("stored %v", getBackend())
	}
	inits := 0
	restore := swapInit(func() error {
		inits++
		return errors.New("non-HID must not init")
	})
	defer restore()
	err := ScrollWheel(0)
	if !errors.Is(err, ErrInvalidScrollSteps) {
		t.Fatalf("ScrollWheel = %v", err)
	}
	if inits != 0 {
		t.Fatal("non-HID backend initialized HID")
	}
	status := ProbeHID()
	if status != HIDUnknown {
		t.Fatalf("ProbeHID under stub = %q", status)
	}
	if getBackend() != 42 {
		t.Fatal("ProbeHID changed the global backend")
	}
}

func TestWindowInputDirectedScroll(t *testing.T) {
	hwnd := createStaticWindow(true)
	if hwnd == 0 {
		t.Fatal("CreateWindowEx failed")
	}
	defer destroyWindow(hwnd)
	w := &Window{HWND: hwnd}
	if err := w.checkReady(); err != nil {
		t.Fatalf("visible static not ready: %v", err)
	}
	executor := &Input{backend: BackendHID, valid: true}
	facade, err := executor.ForWindow(w)
	if err != nil {
		t.Fatal(err)
	}
	err = facade.Scroll(1, 1, 120)
	if !errors.Is(err, ErrDirectedScrollUnsupported) {
		t.Fatalf("HID scroll = %v", err)
	}

	hidden := createStaticWindow(false)
	if hidden == 0 {
		t.Fatal("hidden CreateWindowEx failed")
	}
	defer destroyWindow(hidden)
	hw := &Window{HWND: hidden}
	if hw.IsValid() && hw.IsVisible() {
		t.Fatal("hidden window looks ready")
	}
	if err := hw.Move(0, 0); !errors.Is(err, ErrWindowNotVisible) {
		t.Fatalf("hidden Move = %v, want not visible", err)
	}
	report, err := ProbeTarget(hw)
	if err != nil || report == nil {
		t.Fatalf("hidden probe = %v, %v", report, err)
	}
	if report.Features.TargetClass != "Static" {
		t.Fatalf("class = %q", report.Features.TargetClass)
	}
	if strings.ContainsAny(report.Features.ProcessName, `\/:`) {
		t.Fatalf("process name is a path: %q", report.Features.ProcessName)
	}
	if report.Features.IntegrityRelation == "" || report.Features.CallerUIAccess == "" {
		t.Fatalf("empty tokens %+v", report.Features)
	}
	if !reflect.DeepEqual(report.Warnings, deriveWarnings(report.Features.IntegrityRelation, report.Features.CallerUIAccess)) {
		t.Fatalf("warnings = %#v", report.Warnings)
	}
	if _, ok := reflect.TypeOf(*report).FieldByName("SuggestedBackend"); ok {
		t.Fatal("probe has a suggestion field")
	}
}

func assertNoSuggestionFields(t *testing.T) {
	t.Helper()
	rt := reflect.TypeOf(TargetProbe{})
	for _, name := range []string{"SuggestedBackend", "HWND", "Confidence", "HIDStatus"} {
		if _, ok := rt.FieldByName(name); ok {
			t.Fatalf("TargetProbe has %s", name)
		}
	}
}

func swapInit(fn func() error) func() {
	prev := callHIDInit
	callHIDInit = fn
	return func() { callHIDInit = prev }
}

func swapIdentity(fn func(uintptr) idSnap) func() {
	prev := readIdentity
	readIdentity = fn
	return func() { readIdentity = prev }
}

func swapFacts(fn func(syscall.Handle, bool) (string, IntegrityRelation, UIAccess)) func() {
	prev := collectFacts
	collectFacts = fn
	return func() { collectFacts = prev }
}

func swapOpenProcess(fn func(uint32) (syscall.Handle, bool)) func() {
	prev := openTargetProcess
	openTargetProcess = fn
	return func() { openTargetProcess = prev }
}

func swapCloseProcess(fn func(syscall.Handle)) func() {
	prev := closeTargetProcess
	closeTargetProcess = fn
	return func() { closeTargetProcess = prev }
}

func swapTargetName(fn func(syscall.Handle) string) func() {
	prev := readTargetProcessName
	readTargetProcessName = fn
	return func() { readTargetProcessName = prev }
}

func swapTargetToken(fn func(uintptr) (syscall.Handle, bool)) func() {
	prev := openTargetToken
	openTargetToken = fn
	return func() { openTargetToken = prev }
}

func swapTargetIntegrity(fn func(syscall.Handle) (uint32, bool)) func() {
	prev := readTargetIntegrityRID
	readTargetIntegrityRID = fn
	return func() { readTargetIntegrityRID = prev }
}

func swapTargetTokenClose(fn func(syscall.Handle)) func() {
	prev := closeTargetProcessToken
	closeTargetProcessToken = fn
	return func() { closeTargetProcessToken = prev }
}

func createStaticWindow(visible bool) uintptr {
	class, err := syscall.UTF16PtrFromString("Static")
	if err != nil {
		return 0
	}
	var style uintptr
	off := int32(-32000)
	x := uintptr(off)
	y := uintptr(off)
	if visible {
		style = 0x80000000 | 0x10000000 // WS_POPUP | WS_VISIBLE
	}
	proc := syscall.NewLazyDLL("user32.dll").NewProc("CreateWindowExW")
	hwnd, _, _ := proc.Call(0, uintptr(unsafe.Pointer(class)), 0, style, x, y, 10, 10, 0, 0, 0, 0)
	return hwnd
}

func destroyWindow(hwnd uintptr) {
	proc := syscall.NewLazyDLL("user32.dll").NewProc("DestroyWindow")
	_, _, _ = proc.Call(hwnd)
}
