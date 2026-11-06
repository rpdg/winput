package window

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

func utf16Ptr(s string) (*uint16, error) {
	return syscall.UTF16PtrFromString(s)
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

// FindByPID returns all top-level windows belonging to the specified Process ID.
func FindByPID(targetPid uint32) ([]uintptr, error) {
	var hwnds []uintptr

	cb := syscall.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
		var pid uint32
		ProcGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))

		if pid == targetPid {
			hwnds = append(hwnds, hwnd)
		}
		return 1 // Continue enumeration
	})

	r, _, e := ProcEnumWindows.Call(cb, 0)
	if r == 0 {
		// EnumWindows returns 0 if it fails OR if the callback stops it.
		// Since our callback always returns 1, r==0 implies failure or no windows (unlikely).
		// Check LastError.
		if errno, ok := e.(syscall.Errno); ok && errno != 0 {
			return nil, fmt.Errorf("EnumWindows failed: %w", errno)
		}
	}

	if len(hwnds) == 0 {
		return nil, fmt.Errorf("no windows found for PID: %d", targetPid)
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
// is case-insensitive.
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
