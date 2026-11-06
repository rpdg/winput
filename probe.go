package winput

import (
	"errors"
	"path/filepath"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/rpdg/winput/hid"
	"github.com/rpdg/winput/window"
)

// IntegrityRelation is the caller's view of the target process integrity.
// The zero value and the empty string both mean unknown. A successful probe
// returns the word "unknown", never an empty string.
type IntegrityRelation string

const (
	IntegrityUnknown      IntegrityRelation = "unknown"
	IntegrityTargetHigher IntegrityRelation = "target_higher"
	IntegrityEqual        IntegrityRelation = "equal"
	IntegrityTargetLower  IntegrityRelation = "target_lower"
)

// UIAccess is the caller token's UIAccess flag.
// The zero value and the empty string both mean unknown.
type UIAccess string

const (
	UIAccessUnknown  UIAccess = "unknown"
	UIAccessEnabled  UIAccess = "enabled"
	UIAccessDisabled UIAccess = "disabled"
)

// HIDStatus is the result of initializing this process's HID backend.
// The zero value and the empty string both mean unknown. ProbeHID never
// returns an empty string.
type HIDStatus string

const (
	HIDUnknown            HIDStatus = "unknown"
	HIDInitialized        HIDStatus = "initialized"
	HIDDLLUnavailable     HIDStatus = "dll_unavailable"
	HIDDriverUnavailable  HIDStatus = "driver_unavailable"
	HIDDevicesUnavailable HIDStatus = "devices_unavailable"
)

// ProbeWarning is a risk derived from probe facts. It is not a statement
// that input was blocked.
type ProbeWarning string

const (
	WarnUIPILikelyBlocked ProbeWarning = "uipi_likely_blocked"
	WarnIntegrityUnknown  ProbeWarning = "integrity_unknown"
	WarnUIAccessUnknown   ProbeWarning = "uiaccess_unknown"
)

// TargetFeatures are observations about one window and this process.
// ProcessName is an executable file name, never a path. TargetClass is the
// class of that window itself.
type TargetFeatures struct {
	ProcessName       string
	TargetClass       string
	IntegrityRelation IntegrityRelation
	CallerUIAccess    UIAccess
}

// TargetProbe is a fact report. It does not recommend a backend and does not
// include an HID status.
type TargetProbe struct {
	Features TargetFeatures
	Warnings []ProbeWarning
}

type classSnap struct {
	known bool
	value string
}

type idSnap struct {
	valid bool
	idOK  bool
	pid   uint32
	tid   uint32
	class classSnap
}

// deriveWarnings lists the closed warning set in a fixed order.
// target_higher and unknown cannot both be set, so the two integrity
// warnings never appear together. enabled does not cancel the UIPI warning.
func deriveWarnings(rel IntegrityRelation, access UIAccess) []ProbeWarning {
	if rel == "" {
		rel = IntegrityUnknown
	}
	if access == "" {
		access = UIAccessUnknown
	}
	var out []ProbeWarning
	switch rel {
	case IntegrityTargetHigher:
		out = append(out, WarnUIPILikelyBlocked)
	case IntegrityUnknown:
		out = append(out, WarnIntegrityUnknown)
	}
	if access == UIAccessUnknown {
		out = append(out, WarnUIAccessUnknown)
	}
	return out
}

// classifyHID maps an initialization result onto a status. The order is the
// contract: driver, then devices, then library or symbols, then unknown.
func classifyHID(err error) HIDStatus {
	if err == nil {
		return HIDInitialized
	}
	switch {
	case errors.Is(err, hid.ErrDriverNotInstalled):
		return HIDDriverUnavailable
	case errors.Is(err, hid.ErrDevicesUnavailable):
		return HIDDevicesUnavailable
	case errors.Is(err, hid.ErrLibraryNotFound), errors.Is(err, hid.ErrSymbolsMissing):
		return HIDDLLUnavailable
	default:
		return HIDUnknown
	}
}

// resolveIdentity applies the class-name and identity table.
// A class read that returned a value, including an empty string, is known.
func resolveIdentity(start, end idSnap) (string, error) {
	if !start.valid || !end.valid || !start.idOK || !end.idOK {
		return "", ErrWindowGone
	}
	if start.pid != end.pid || start.tid != end.tid {
		return "", ErrTargetChanged
	}
	switch {
	case start.class.known && end.class.known && start.class.value != end.class.value:
		return "", ErrTargetChanged
	case start.class.known && !end.class.known:
		return start.class.value, nil
	case !start.class.known && end.class.known:
		return end.class.value, nil
	case start.class.known && end.class.known:
		return start.class.value, nil
	default:
		return "", nil
	}
}

// readIdentity is the Win32 snapshot used by ProbeTarget. Tests replace it.
var readIdentity = readWindowIdentity

// collectFacts reads the process name, integrity relation, and caller UIAccess.
// Tests replace it. The default closes every process handle it opens.
var collectFacts = readTargetFacts

// ProbeTarget reports facts about w. It does not initialize HID, take the
// input lock, or change the global backend. An invalid window or a window
// whose process or thread id cannot be read returns ErrWindowGone and no report.
func ProbeTarget(w *Window) (*TargetProbe, error) {
	if w == nil {
		return nil, ErrWindowGone
	}
	// Hold the handle taken at entry. A caller that mutates w.HWND while the
	// process token is being read must not splice that new handle into the end
	// snapshot. Facts are read only after a usable start snapshot, and the end
	// snapshot is taken after those reads. A mismatch discards the facts.
	hwnd := w.HWND
	start := readIdentity(hwnd)
	if !start.valid || !start.idOK {
		return nil, ErrWindowGone
	}
	name, rel, access := collectFacts(start.pid)
	end := readIdentity(hwnd)
	class, err := resolveIdentity(start, end)
	if err != nil {
		return nil, err
	}
	if rel == "" {
		rel = IntegrityUnknown
	}
	if access == "" {
		access = UIAccessUnknown
	}
	return &TargetProbe{
		Features: TargetFeatures{
			ProcessName:       name,
			TargetClass:       class,
			IntegrityRelation: rel,
			CallerUIAccess:    access,
		},
		Warnings: deriveWarnings(rel, access),
	}, nil
}

// ProbeHID reports whether this process can initialize HID.
// Initialization failure still returns a status. The global backend is not read
// or changed, and the input lock is not taken.
func ProbeHID() HIDStatus {
	return classifyHID(callHIDInit())
}

func readWindowIdentity(hwnd uintptr) idSnap {
	snap := idSnap{valid: window.IsValid(hwnd)}
	if !snap.valid {
		return snap
	}
	var pid uint32
	tid, _, _ := window.ProcGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if tid == 0 {
		return snap
	}
	snap.idOK = true
	snap.pid = pid
	snap.tid = uint32(tid)
	if class, ok := readClassName(hwnd); ok {
		snap.class = classSnap{known: true, value: class}
	}
	return snap
}

func readClassName(hwnd uintptr) (string, bool) {
	buf := make([]uint16, 256)
	n, _, _ := window.ProcGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return "", false
	}
	return string(utf16.Decode(buf[:n])), true
}

const (
	processQueryLimited = 0x1000
	tokenQuery          = 0x0008
	tokenIntegrityLevel = 25
	tokenUIAccessClass  = 26
)

func readTargetFacts(pid uint32) (string, IntegrityRelation, UIAccess) {
	access, callerRID, callerOK := readCallerToken()
	name, targetRID, targetOK := readProcessToken(pid)
	return name, integrityRelation(callerRID, callerOK, targetRID, targetOK), access
}

func integrityRelation(caller uint32, callerOK bool, target uint32, targetOK bool) IntegrityRelation {
	if !callerOK || !targetOK {
		return IntegrityUnknown
	}
	switch {
	case target > caller:
		return IntegrityTargetHigher
	case target < caller:
		return IntegrityTargetLower
	default:
		return IntegrityEqual
	}
}

func readCallerToken() (UIAccess, uint32, bool) {
	proc, _, _ := window.ProcGetCurrentProcess.Call()
	if proc == 0 {
		return UIAccessUnknown, 0, false
	}
	token, ok := openToken(proc)
	if !ok {
		return UIAccessUnknown, 0, false
	}
	defer closeOSHandle(token)
	rid, ridOK := tokenIntegrityRID(token)
	return tokenUIAccess(token), rid, ridOK
}

func readProcessToken(pid uint32) (string, uint32, bool) {
	if pid == 0 {
		return "", 0, false
	}
	proc, _, _ := window.ProcOpenProcess.Call(processQueryLimited, 0, uintptr(pid))
	if proc == 0 {
		return "", 0, false
	}
	defer closeOSHandle(syscall.Handle(proc))
	name := processFileName(syscall.Handle(proc))
	token, ok := openToken(proc)
	if !ok {
		return name, 0, false
	}
	defer closeOSHandle(token)
	rid, ridOK := tokenIntegrityRID(token)
	return name, rid, ridOK
}

func openToken(proc uintptr) (syscall.Handle, bool) {
	var token syscall.Handle
	r, _, _ := window.ProcOpenProcessToken.Call(proc, tokenQuery, uintptr(unsafe.Pointer(&token)))
	if r == 0 {
		return 0, false
	}
	return token, true
}

func closeOSHandle(h syscall.Handle) {
	if h != 0 {
		_, _, _ = window.ProcCloseHandle.Call(uintptr(h))
	}
}

func tokenUIAccess(token syscall.Handle) UIAccess {
	var flag uint32
	var needed uint32
	r, _, _ := window.ProcGetTokenInformation.Call(
		uintptr(token),
		tokenUIAccessClass,
		uintptr(unsafe.Pointer(&flag)),
		unsafe.Sizeof(flag),
		uintptr(unsafe.Pointer(&needed)),
	)
	if r == 0 {
		return UIAccessUnknown
	}
	if flag != 0 {
		return UIAccessEnabled
	}
	return UIAccessDisabled
}

func tokenIntegrityRID(token syscall.Handle) (uint32, bool) {
	var needed uint32
	_, _, _ = window.ProcGetTokenInformation.Call(uintptr(token), tokenIntegrityLevel, 0, 0, uintptr(unsafe.Pointer(&needed)))
	if needed < uint32(unsafe.Sizeof(uintptr(0))) {
		return 0, false
	}
	buf := make([]byte, needed)
	r, _, _ := window.ProcGetTokenInformation.Call(
		uintptr(token),
		tokenIntegrityLevel,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&needed)),
	)
	if r == 0 {
		return 0, false
	}
	sid := *(*uintptr)(unsafe.Pointer(&buf[0]))
	if sid == 0 {
		return 0, false
	}
	countPtr, _, _ := window.ProcGetSidSubAuthorityCount.Call(sid)
	if countPtr == 0 {
		return 0, false
	}
	count := *(*uint8)(unsafe.Pointer(countPtr))
	if count == 0 {
		return 0, false
	}
	sub, _, _ := window.ProcGetSidSubAuthority.Call(sid, uintptr(count-1))
	if sub == 0 {
		return 0, false
	}
	return *(*uint32)(unsafe.Pointer(sub)), true
}

func processFileName(proc syscall.Handle) string {
	buf := make([]uint16, 32768)
	size := uint32(len(buf))
	r, _, _ := window.ProcQueryFullProcessImageNameW.Call(
		uintptr(proc),
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if r == 0 || size == 0 || int(size) > len(buf) {
		return ""
	}
	name := filepath.Base(string(utf16.Decode(buf[:size])))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return ""
	}
	return name
}
