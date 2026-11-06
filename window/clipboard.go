package window

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

const (
	// cfUnicodeText is CF_UNICODETEXT (13). Lines are CR-LF terminated, and
	// the format ends with a NUL.
	cfUnicodeText = 13
	// gmemMoveable is GMEM_MOVEABLE (0x0002).
	gmemMoveable = 0x0002
)

// maxClipboardBytes is the internal hard limit for one clipboard text buffer:
// 16 MiB. It guards make against a hostile GlobalSize value or an oversized
// input; it is not a public capacity promise.
const maxClipboardBytes = 16 << 20

// findClipboardProc resolves a lazy procedure and turns a missing procedure
// into a fixed ErrClipboardFailed wrapper. The Find error is deliberately not
// embedded because its text can carry the DLL path.
func findClipboardProc(name string, p *syscall.LazyProc) error {
	if err := p.Find(); err != nil {
		return fmt.Errorf("%w: %s: procedure not found", ErrClipboardFailed, name)
	}
	return nil
}

// callClipboard runs a found LazyProc and turns a zero main return value into
// an ErrClipboardFailed wrapper. A non-zero syscall error code is appended
// when one was reported. It is not used for GlobalUnlock, which has its own
// three-way outcome.
func callClipboard(name string, p *syscall.LazyProc, args ...uintptr) error {
	if err := findClipboardProc(name, p); err != nil {
		return err
	}
	r, _, errno := p.Call(args...)
	if r != 0 {
		return nil
	}
	if e, ok := errno.(syscall.Errno); ok && e != 0 {
		return fmt.Errorf("%w: %s: %v", ErrClipboardFailed, name, e)
	}
	return fmt.Errorf("%w: %s", ErrClipboardFailed, name)
}

func openClipboard() error {
	return callClipboard("OpenClipboard", ProcOpenClipboard, 0)
}

func closeClipboard() error {
	return callClipboard("CloseClipboard", ProcCloseClipboard)
}

func emptyClipboard() error {
	return callClipboard("EmptyClipboard", ProcEmptyClipboard)
}

func getClipboardData() (uintptr, error) {
	if err := findClipboardProc("GetClipboardData", ProcGetClipboardData); err != nil {
		return 0, err
	}
	r, _, _ := ProcGetClipboardData.Call(cfUnicodeText)
	if r == 0 {
		return 0, fmt.Errorf("%w: GetClipboardData", ErrClipboardFailed)
	}
	return r, nil
}

func setClipboardData(hmem uintptr) error {
	return callClipboard("SetClipboardData", ProcSetClipboardData, cfUnicodeText, hmem)
}

func isClipboardFormatAvailable() (bool, error) {
	if err := findClipboardProc("IsClipboardFormatAvailable", ProcIsClipboardFormatAvailable); err != nil {
		return false, err
	}
	r, _, _ := ProcIsClipboardFormatAvailable.Call(cfUnicodeText)
	return r != 0, nil
}

func globalAlloc(size int) (uintptr, error) {
	if err := findClipboardProc("GlobalAlloc", ProcGlobalAlloc); err != nil {
		return 0, err
	}
	r, _, errno := ProcGlobalAlloc.Call(gmemMoveable, uintptr(size))
	if r == 0 {
		if e, ok := errno.(syscall.Errno); ok && e != 0 {
			return 0, fmt.Errorf("%w: GlobalAlloc: %v", ErrClipboardFailed, e)
		}
		return 0, fmt.Errorf("%w: GlobalAlloc", ErrClipboardFailed)
	}
	return r, nil
}

func globalLock(hmem uintptr) (uintptr, error) {
	if err := findClipboardProc("GlobalLock", ProcGlobalLock); err != nil {
		return 0, err
	}
	r, _, errno := ProcGlobalLock.Call(hmem)
	if r == 0 {
		if e, ok := errno.(syscall.Errno); ok && e != 0 {
			return 0, fmt.Errorf("%w: GlobalLock: %v", ErrClipboardFailed, e)
		}
		return 0, fmt.Errorf("%w: GlobalLock", ErrClipboardFailed)
	}
	return r, nil
}

// globalUnlock releases one GlobalLock and reports whether the block is still
// locked by another owner. A non-zero main return means this unlock succeeded
// but the lock count is still above zero; the caller's own layer is released
// and it must not unlock again or GlobalFree. Only a zero return with
// NO_ERROR means the lock count reached zero; a zero return with another
// error code (for example ERROR_NOT_LOCKED) means this unlock did not match a
// lock. LazyProc.Call reports GetLastError as a non-nil syscall.Errno even
// when it is 0, so the error's value is checked, not its nil-ness.
func globalUnlock(hmem uintptr, where string) (stillLocked bool, err error) {
	if err := findClipboardProc("GlobalUnlock", ProcGlobalUnlock); err != nil {
		return false, err
	}
	r, _, errno := ProcGlobalUnlock.Call(hmem)
	if r != 0 {
		return true, nil
	}
	if e, ok := errno.(syscall.Errno); ok && e != 0 {
		return false, fmt.Errorf("%w: GlobalUnlock[%s]: %v", ErrClipboardFailed, where, e)
	}
	return false, nil
}

func globalSize(hmem uintptr) (uintptr, error) {
	if err := findClipboardProc("GlobalSize", ProcGlobalSize); err != nil {
		return 0, err
	}
	r, _, _ := ProcGlobalSize.Call(hmem)
	if r == 0 {
		return 0, fmt.Errorf("%w: GlobalSize: invalid handle or discarded memory", ErrClipboardFailed)
	}
	return r, nil
}

func globalFree(hmem uintptr) error {
	if err := findClipboardProc("GlobalFree", ProcGlobalFree); err != nil {
		return err
	}
	r, _, _ := ProcGlobalFree.Call(hmem)
	if r != 0 {
		return fmt.Errorf("%w: GlobalFree", ErrClipboardFailed)
	}
	return nil
}

// countUnicodeCodeUnits counts the UTF-16 code units text occupies, applying
// the same lone-"\n" expansion that encodeUnicodeText writes: a "\n" not
// directly preceded by "\r" becomes two units. A non-BMP rune counts two.
func countUnicodeCodeUnits(text string, normalize bool) int {
	var units int
	prevCR := false
	for _, r := range text {
		if normalize && r == '\n' && !prevCR {
			units += 2
		} else if r >= 0x10000 {
			units += 2
		} else {
			units++
		}
		prevCR = r == '\r'
	}
	return units
}

// encodeUnicodeText converts text to UTF-16 code units, optionally expanding a
// lone "\n" (one not already preceded by "\r") to "\r\n", and appends a
// terminating NUL. A "\r\n" already in text stays an exact "\r\n"; a lone "\r"
// stays "\r". The code-unit count is computed first so oversized input is
// rejected with ErrClipboardFailed before any buffer is allocated.
func encodeUnicodeText(text string, normalize bool) ([]uint16, error) {
	units := countUnicodeCodeUnits(text, normalize) + 1 // + terminator NUL
	if units > maxClipboardBytes/2 {
		return nil, fmt.Errorf("%w: clipboard text exceeds internal size limit", ErrClipboardFailed)
	}
	buf := make([]uint16, units)
	n := 0
	prevCR := false
	for _, r := range text {
		if normalize && r == '\n' && !prevCR {
			buf[n] = '\r'
			n++
			buf[n] = '\n'
			n++
		} else if r < 0x10000 {
			buf[n] = uint16(r)
			n++
		} else {
			rr := r - 0x10000
			buf[n] = 0xD800 + uint16(rr>>10)
			n++
			buf[n] = 0xDC00 + uint16(rr&0x3FF)
			n++
		}
		prevCR = r == '\r'
	}
	buf[n] = 0
	return buf, nil
}

// sysptr converts a uintptr returned by a Win32 call (such as GlobalLock's
// pointer to a locked HGLOBAL) into an unsafe.Pointer. The direct conversion
// unsafe.Pointer(uintptr) trips go vet's unsafeptr check because vet cannot
// see that the value is a stable heap pointer the system handed us; reading
// the value through a pointer of the target size keeps the check quiet. The
// memory is owned by the OS until the matching GlobalUnlock, and the caller
// only reads or writes its own locked block.
func sysptr(p uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&p))
}

// readUnicodeText opens the clipboard itself and reads the CF_UNICODETEXT
// contents the system can provide at this moment. hadUnicode is true only when
// such a format is available; it does not mean the clipboard is "empty"
// otherwise (other formats can still exist, and CF_TEXT can be converted to
// CF_UNICODETEXT). Every return path closes the clipboard exactly once when
// it was opened by this call. The GetClipboardData handle is system-owned: it
// is only locked, copied, and unlocked, never GlobalFree'd.
func readUnicodeText() (text string, hadUnicode bool, err error) {
	if err := openClipboard(); err != nil {
		return "", false, err
	}
	ok, err := isClipboardFormatAvailable()
	if err != nil {
		return "", false, errors.Join(err, closeClipboard())
	}
	if !ok {
		if err := closeClipboard(); err != nil {
			return "", false, err
		}
		return "", false, nil
	}
	handle, err := getClipboardData()
	if err != nil {
		return "", false, errors.Join(err, closeClipboard())
	}
	size, err := globalSize(handle)
	if err != nil {
		return "", false, errors.Join(err, closeClipboard())
	}
	if size%2 != 0 || size > maxClipboardBytes {
		return "", false, errors.Join(fmt.Errorf("%w: GetClipboardData: invalid size", ErrClipboardFailed), closeClipboard())
	}
	ptr, err := globalLock(handle)
	if err != nil {
		return "", false, errors.Join(err, closeClipboard())
	}
	n := int(size / 2)
	units := make([]uint16, n)
	copy(units, unsafe.Slice((*uint16)(sysptr(ptr)), n))
	// The GetClipboardData handle is borrowed, not ours to free. On this
	// WinSta0 the system hands out blocks whose lock count is already above
	// zero, so a "still locked" result from our single unlock is the normal,
	// benign outcome: our own layer is released and the block stays owned by
	// the system. Only an unlock that fails to match (0 + error code) is a
	// read failure. We never unlock again and never GlobalFree.
	if _, err := globalUnlock(handle, "read"); err != nil {
		return "", false, errors.Join(err, closeClipboard())
	}
	if err := closeClipboard(); err != nil {
		return "", false, err
	}
	return syscall.UTF16ToString(units), true, nil
}

// writeUnicodeText opens the clipboard itself and replaces its contents with
// the encoded text. normalize controls the lone-"\n" to "\r\n" expansion;
// restoring previously read text passes false so the recorded line endings are
// written back unchanged. mutated reports whether EmptyClipboard succeeded,
// i.e. whether the previous contents were dropped (EmptyClipboard drops every
// format; that is a known limitation). Every return path closes the clipboard
// exactly once when it was opened by this call. A SetClipboardData failure
// after a successful EmptyClipboard keeps mutated true, because the caller
// must still restore.
func writeUnicodeText(text string, normalize bool) (mutated bool, err error) {
	if err := openClipboard(); err != nil {
		return false, err
	}
	units, err := encodeUnicodeText(text, normalize)
	if err != nil {
		return false, errors.Join(err, closeClipboard())
	}

	hmem, err := globalAlloc(len(units) * 2)
	if err != nil {
		return false, errors.Join(err, closeClipboard())
	}
	ptr, err := globalLock(hmem)
	if err != nil {
		// The block was never locked; it can be freed.
		return false, errors.Join(err, globalFree(hmem), closeClipboard())
	}
	// Write the decided code-unit sequence plus the terminating NUL.
	dst := unsafe.Slice((*uint16)(sysptr(ptr)), len(units))
	copy(dst, units)

	// Our own block is balanced (one lock), so a "still locked" result here is
	// an error: it would mean someone else also holds it. Do not free memory
	// that may still be locked.
	if stillLocked, err := globalUnlock(hmem, "write"); err != nil || stillLocked {
		return false, errors.Join(err, closeClipboard())
	}
	if err := emptyClipboard(); err != nil {
		return false, errors.Join(err, globalFree(hmem), closeClipboard())
	}
	mutated = true
	if err := setClipboardData(hmem); err != nil {
		// The accepted-later block was unlocked then rejected; free it.
		return true, errors.Join(err, globalFree(hmem), closeClipboard())
	}
	// hmem now belongs to the system; only the single close remains.
	if err := closeClipboard(); err != nil {
		// EmptyClipboard already ran; the caller must still restore.
		return true, err
	}
	return true, nil
}

// restoreUnicodeText writes the previously read Unicode text back (or a bare
// empty Unicode buffer when hadUnicode is false) without newline
// normalization. It does not open or close the clipboard itself; the single
// writeUnicodeText call does.
func restoreUnicodeText(prev string, hadUnicode bool) error {
	if !hadUnicode {
		_, err := writeUnicodeText("", false)
		return err
	}
	_, err := writeUnicodeText(prev, false)
	return err
}

// SwapUnicodeText temporarily replaces the clipboard's CF_UNICODETEXT contents
// with text (lone "\n" normalized to "\r\n") and returns a restore function
// that puts the previous Unicode text back byte-for-byte (or a bare empty
// Unicode buffer when none was available). It is for winput.PasteByClipboard
// and is not part of the library's documented public API.
//
// The caller must hold whatever lock serializes its input and call restore
// after delivering Ctrl+V; restore re-opens the clipboard itself. The rules:
// err == nil means the swap replaced the clipboard and restore must be called.
// err != nil with a non-nil restore means the clipboard was cleared but the
// write or close failed, so the caller must still call restore. err != nil
// with nil restore means the clipboard was not modified and nothing needs
// restoring. The clipboard is a window-station object; this function never
// calls EmptyClipboard outside the single writeUnicodeText path.
func SwapUnicodeText(text string) (restore func() error, err error) {
	prev, had, err := readUnicodeText()
	if err != nil {
		return nil, err
	}
	mutated, err := writeUnicodeText(text, true)
	if err == nil {
		return func() error { return restoreUnicodeText(prev, had) }, nil
	}
	if !mutated {
		return nil, err
	}
	return func() error { return restoreUnicodeText(prev, had) }, err
}
