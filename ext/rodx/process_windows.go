package rodx

import (
	"fmt"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"github.com/rpdg/winput/window"
)

var (
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess                = kernel32.NewProc("OpenProcess")
	procQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
	procTerminateProcess           = kernel32.NewProc("TerminateProcess")
	procWaitForSingleObject        = kernel32.NewProc("WaitForSingleObject")
)

const (
	processTerminate               = 0x0001
	processQueryLimitedInformation = 0x1000
	synchronizeAccess              = 0x00100000

	waitObject0 = 0x00000000
	waitTimeout = 0x00000102

	wmClose = 0x0010
)

func pidFromHWND(hwnd uintptr) (uint32, error) {
	var pid uint32
	window.ProcGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return 0, fmt.Errorf("failed to get pid for hwnd 0x%x", hwnd)
	}
	return pid, nil
}

func openProcess(access uint32, pid uint32) (uintptr, error) {
	handle, _, err := procOpenProcess.Call(uintptr(access), 0, uintptr(pid))
	if handle == 0 {
		return 0, fmt.Errorf("OpenProcess failed for pid %d: %v", pid, err)
	}
	return handle, nil
}

func executableFromPID(pid uint32) (string, error) {
	handle, err := openProcess(processQueryLimitedInformation, pid)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrRestartSpecIncomplete, err)
	}
	defer window.ProcCloseHandle.Call(handle)

	buf := make([]uint16, syscall.MAX_PATH)
	size := uint32(len(buf))
	r, _, callErr := procQueryFullProcessImageNameW.Call(
		handle,
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if r == 0 {
		return "", fmt.Errorf("QueryFullProcessImageNameW failed for pid %d: %v", pid, callErr)
	}

	return syscall.UTF16ToString(buf[:size]), nil
}

func workingDirFromExecutable(executable string) string {
	if executable == "" {
		return ""
	}
	return filepath.Dir(executable)
}

func closeWindowsForPID(pid uint32) {
	hwnds, err := window.FindByPID(pid)
	if err != nil {
		return
	}
	for _, hwnd := range hwnds {
		window.ProcPostMessageW.Call(hwnd, wmClose, 0, 0)
	}
}

func waitForExit(pid uint32, timeout time.Duration) error {
	handle, err := openProcess(synchronizeAccess, pid)
	if err != nil {
		return err
	}
	defer window.ProcCloseHandle.Call(handle)

	ms := uint32(timeout / time.Millisecond)
	if timeout <= 0 {
		ms = 0
	}
	r, _, callErr := procWaitForSingleObject.Call(handle, uintptr(ms))
	switch uint32(r) {
	case waitObject0:
		return nil
	case waitTimeout:
		return ErrProcessShutdownFailed
	default:
		return fmt.Errorf("WaitForSingleObject failed for pid %d: %v", pid, callErr)
	}
}

func terminatePID(pid uint32) error {
	handle, err := openProcess(processTerminate|synchronizeAccess, pid)
	if err != nil {
		return err
	}
	defer window.ProcCloseHandle.Call(handle)

	r, _, callErr := procTerminateProcess.Call(handle, 1)
	if r == 0 {
		return fmt.Errorf("TerminateProcess failed for pid %d: %v", pid, callErr)
	}
	return nil
}
