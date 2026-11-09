package window

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

func utf16Ptr(s string) (*uint16, error) {
	ptr, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidLookupString, err)
	}
	return ptr, nil
}

// FindByTitle searches for a top-level window matching the exact title.
func FindByTitle(title string) (uintptr, error) {
	titlePtr, err := utf16Ptr(title)
	if err != nil {
		return 0, fmt.Errorf("invalid window title: %w", err)
	}
	ret, _, _ := ProcFindWindowW.Call(
		0,
		uintptr(unsafe.Pointer(titlePtr)),
	)
	if ret == 0 {
		return 0, fmt.Errorf("window not found with title: %s", title)
	}
	return ret, nil
}

// FindByClass searches for a top-level window matching the specified class name.
func FindByClass(class string) (uintptr, error) {
	classPtr, err := utf16Ptr(class)
	if err != nil {
		return 0, fmt.Errorf("invalid window class: %w", err)
	}
	ret, _, _ := ProcFindWindowW.Call(
		uintptr(unsafe.Pointer(classPtr)),
		0,
	)
	if ret == 0 {
		return 0, fmt.Errorf("window not found with class: %s", class)
	}
	return ret, nil
}

// FindChildByClass searches for a child window with the specified class name.
func FindChildByClass(parent uintptr, class string) (uintptr, error) {
	classPtr, err := utf16Ptr(class)
	if err != nil {
		return 0, fmt.Errorf("invalid window class: %w", err)
	}
	ret, _, _ := ProcFindWindowExW.Call(
		parent,
		0,
		uintptr(unsafe.Pointer(classPtr)),
		0,
	)
	if ret == 0 {
		return 0, fmt.Errorf("child window not found with class: %s", class)
	}
	return ret, nil
}

// hwndVisitor is one EnumWindows callback. The pointer is published with an
// atomic so the single registered thunk can see it. The callback runs on the
// calling thread while enumMu is held, and it must not call enumTopLevel.
type hwndVisitor func(hwnd uintptr)

var (
	enumMu      sync.Mutex
	enumVisit   atomic.Pointer[hwndVisitor]
	enumWindows = syscall.NewCallback(enumWindowsThunk)
)

func enumWindowsThunk(hwnd uintptr, _ uintptr) uintptr {
	visit := enumVisit.Load()
	if visit != nil {
		(*visit)(hwnd)
	}
	return 1
}

// enumTopLevel visits every top-level window. Concurrent callers wait, because
// one process-wide callback slot carries the visitor.
func enumTopLevel(visit hwndVisitor) error {
	enumMu.Lock()
	defer enumMu.Unlock()
	// Heap copy: the thunk reads this pointer while EnumWindows is inside
	// the syscall, and a stack address would not stay valid across that call.
	held := new(hwndVisitor)
	*held = visit
	enumVisit.Store(held)
	defer enumVisit.Store(nil)

	r, _, e := ProcEnumWindows.Call(enumWindows, 0)
	if r == 0 {
		// The thunk always returns 1, so 0 means EnumWindows itself failed.
		if errno, ok := e.(syscall.Errno); ok && errno != 0 {
			return fmt.Errorf("EnumWindows failed: %w", errno)
		}
		return fmt.Errorf("EnumWindows failed")
	}
	return nil
}

// FindByPID returns all top-level windows belonging to the specified Process ID.
func FindByPID(targetPid uint32) ([]uintptr, error) {
	var hwnds []uintptr
	err := enumTopLevel(func(hwnd uintptr) {
		var pid uint32
		ProcGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if pid == targetPid {
			hwnds = append(hwnds, hwnd)
		}
	})
	if err != nil {
		return nil, err
	}
	if len(hwnds) == 0 {
		return nil, fmt.Errorf("no windows found for PID: %d", targetPid)
	}
	return hwnds, nil
}

// FindByPIDs returns every top-level window whose process is in pids.
// One EnumWindows pass covers the whole set. An empty pids list or a set
// that owns no windows returns an empty slice and a nil error. Enumeration
// failure returns the EnumWindows error.
func FindByPIDs(pids []uint32) ([]uintptr, error) {
	if len(pids) == 0 {
		return nil, nil
	}
	want := make(map[uint32]struct{}, len(pids))
	for _, pid := range pids {
		want[pid] = struct{}{}
	}
	var hwnds []uintptr
	err := enumTopLevel(func(hwnd uintptr) {
		var pid uint32
		ProcGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if _, ok := want[pid]; ok {
			hwnds = append(hwnds, hwnd)
		}
	})
	if err != nil {
		return nil, err
	}
	return hwnds, nil
}

// Process Enumeration helpers

const (
	TH32CS_SNAPPROCESS = 0x00000002
)

type PROCESSENTRY32 struct {
	Size            uint32
	CntUsage        uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	CntThreads      uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [260]uint16
}

// processEntry is one process in a toolhelp snapshot.
type processEntry struct {
	pid uint32
	exe string
}

// matchingPIDs returns every PID whose executable matches name.
// The comparison is case-insensitive. A name without ".exe" matches the
// ".exe" form. Snapshot order is preserved.
func matchingPIDs(entries []processEntry, name string) []uint32 {
	target := strings.ToLower(name)
	if !strings.HasSuffix(target, ".exe") {
		target += ".exe"
	}
	var pids []uint32
	for _, e := range entries {
		if strings.EqualFold(e.exe, target) {
			pids = append(pids, e.pid)
		}
	}
	return pids
}

// FindPIDsByName returns every process ID with the given executable name
// (for example "notepad.exe"), in toolhelp snapshot order. The comparison
// is case-insensitive. winput.FindByProcessName uses this so a name shared
// by several processes is not reduced to the first snapshot entry.
func FindPIDsByName(name string) ([]uint32, error) {
	const INVALID_HANDLE_VALUE = ^uintptr(0)

	snap, _, err := ProcCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if snap == INVALID_HANDLE_VALUE {
		return nil, fmt.Errorf("CreateToolhelp32Snapshot failed: %v", err)
	}
	defer ProcCloseHandle.Call(snap)

	var pe32 PROCESSENTRY32
	pe32.Size = uint32(unsafe.Sizeof(pe32))

	r, _, err := ProcProcess32First.Call(snap, uintptr(unsafe.Pointer(&pe32)))
	if r == 0 {
		return nil, fmt.Errorf("Process32First failed: %v", err)
	}

	var entries []processEntry
	for {
		entries = append(entries, processEntry{
			pid: pe32.ProcessID,
			exe: syscall.UTF16ToString(pe32.ExeFile[:]),
		})
		r, _, _ = ProcProcess32Next.Call(snap, uintptr(unsafe.Pointer(&pe32)))
		if r == 0 {
			break
		}
	}

	pids := matchingPIDs(entries, name)
	if len(pids) == 0 {
		return nil, fmt.Errorf("process not found: %s", name)
	}
	return pids, nil
}

// FindPIDByName searches for a process ID by its executable name (e.g., "notepad.exe").
// The comparison is case-insensitive. When several processes share the name,
// this returns the first one in toolhelp order. Use FindPIDsByName to collect
// every match.
func FindPIDByName(name string) (uint32, error) {
	pids, err := FindPIDsByName(name)
	if err != nil {
		return 0, err
	}
	return pids[0], nil
}
