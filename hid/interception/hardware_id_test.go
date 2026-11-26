package interception

import (
	"encoding/binary"
	"errors"
	"reflect"
	"syscall"
	"testing"
	"unicode/utf16"
)

func encodeUTF16LEMultiSZ(entries ...string) []byte {
	var units []uint16
	for _, s := range entries {
		units = append(units, utf16.Encode([]rune(s))...)
		units = append(units, 0)
	}
	units = append(units, 0)
	out := make([]byte, len(units)*2)
	for i, u := range units {
		binary.LittleEndian.PutUint16(out[i*2:i*2+2], u)
	}
	return out
}

func TestDecodeHardwareIDs(t *testing.T) {
	t.Run("empty returns nil without error", func(t *testing.T) {
		got, err := decodeHardwareIDs(nil)
		if err != nil || got != nil {
			t.Fatalf("decodeHardwareIDs(nil) = (%#v, %v), want (nil, nil)", got, err)
		}
	})

	t.Run("standard REG_MULTI_SZ", func(t *testing.T) {
		raw := encodeUTF16LEMultiSZ(
			`HID\ddhid63340&Col07`,
			`HID\VID_9AC5&UP:0001_U:0006`,
			`HID_DEVICE_SYSTEM_KEYBOARD`,
		)
		got, err := decodeHardwareIDs(raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{
			`HID\ddhid63340&Col07`,
			`HID\VID_9AC5&UP:0001_U:0006`,
			`HID_DEVICE_SYSTEM_KEYBOARD`,
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})

	t.Run("leading NUL does not hide trailing identifiers", func(t *testing.T) {
		raw := append([]byte{0, 0}, encodeUTF16LEMultiSZ(`HID\VID_046D&PID_C31C`, `HID\ddhid63340&Col03`)...)
		got, err := decodeHardwareIDs(raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{`HID\VID_046D&PID_C31C`, `HID\ddhid63340&Col03`}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})

	t.Run("trailing entry without NUL terminator", func(t *testing.T) {
		units := utf16.Encode([]rune(`ACPI\LEN0071`))
		raw := make([]byte, len(units)*2)
		for i, u := range units {
			binary.LittleEndian.PutUint16(raw[i*2:i*2+2], u)
		}
		got, err := decodeHardwareIDs(raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{`ACPI\LEN0071`}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})

	t.Run("odd byte length rejected", func(t *testing.T) {
		if _, err := decodeHardwareIDs([]byte{0x48, 0x00, 0x49}); err == nil {
			t.Fatal("expected error for odd byte length")
		}
	})

	t.Run("non-empty all-NUL or whitespace buffer rejected", func(t *testing.T) {
		if _, err := decodeHardwareIDs([]byte{0, 0, 0, 0}); err == nil {
			t.Fatal("expected error for NUL-only buffer")
		}
		if _, err := decodeHardwareIDs(encodeUTF16LEMultiSZ("   ")); err == nil {
			t.Fatal("expected error for whitespace-only buffer")
		}
	})
}

func saveAndResetInterceptionGlobals(t *testing.T) {
	t.Helper()
	origLoad := loadLibraryFn
	origFree := freeLibraryFn
	origGetProc := getProcFn
	origSyscall := hardwareIDSyscall
	origPath := libraryPath

	libraryMu.Lock()
	if dllHandle != 0 {
		_ = freeLibraryFn(dllHandle)
		dllHandle = 0
	}
	clearProcsLocked()
	libraryMu.Unlock()

	t.Cleanup(func() {
		libraryMu.Lock()
		if dllHandle != 0 {
			_ = freeLibraryFn(dllHandle)
			dllHandle = 0
		}
		clearProcsLocked()
		libraryPath = origPath
		libraryMu.Unlock()

		loadLibraryFn = origLoad
		freeLibraryFn = origFree
		getProcFn = origGetProc
		hardwareIDSyscall = origSyscall
	})
}

func TestLoadMissingExportsCleansUpAllSixProcs(t *testing.T) {
	allSymbols := []string{
		"interception_create_context",
		"interception_destroy_context",
		"interception_is_mouse",
		"interception_is_keyboard",
		"interception_send",
		"interception_get_hardware_id",
	}

	for _, missing := range allSymbols {
		t.Run("missing_"+missing, func(t *testing.T) {
			saveAndResetInterceptionGlobals(t)

			var freedHandles []syscall.Handle
			loadLibraryFn = func(path string) (syscall.Handle, error) {
				return syscall.Handle(0xCAFE), nil
			}
			freeLibraryFn = func(h syscall.Handle) error {
				freedHandles = append(freedHandles, h)
				return nil
			}
			getProcFn = func(h syscall.Handle, name string) uintptr {
				if name == missing {
					return 0
				}
				return 0x1000 + uintptr(len(name))
			}

			err := Load()
			if !errors.Is(err, ErrSymbolsMissing) {
				t.Fatalf("Load() error = %v, want ErrSymbolsMissing", err)
			}
			if len(freedHandles) != 1 || freedHandles[0] != syscall.Handle(0xCAFE) {
				t.Fatalf("FreeLibrary calls = %v, want [0xCAFE]", freedHandles)
			}
			if dllHandle != 0 {
				t.Fatalf("dllHandle = %v, want 0 after failure", dllHandle)
			}
			if procCreateContext != 0 || procDestroyContext != 0 ||
				procIsMouse != 0 || procIsKeyboard != 0 ||
				procSend != 0 || procGetHardwareID != 0 {
				t.Fatalf("expected all 6 proc pointers zeroed on failure, got (%x,%x,%x,%x,%x,%x)",
					procCreateContext, procDestroyContext, procIsMouse, procIsKeyboard, procSend, procGetHardwareID)
			}
		})
	}
}

func TestGetHardwareIDBoundsAndRetry(t *testing.T) {
	t.Run("missing export returns ErrSymbolsMissing", func(t *testing.T) {
		saveAndResetInterceptionGlobals(t)
		got, err := GetHardwareID(1, 1)
		if !errors.Is(err, ErrSymbolsMissing) || got != nil {
			t.Fatalf("GetHardwareID() = (%v, %v), want (nil, ErrSymbolsMissing)", got, err)
		}
	})

	t.Run("empty slot 0 bytes", func(t *testing.T) {
		saveAndResetInterceptionGlobals(t)
		procGetHardwareID = 0x1234
		hardwareIDSyscall = func(proc uintptr, ctx Context, dev Device, buf []byte) uint32 {
			return 0
		}
		got, err := GetHardwareID(1, 3)
		if err != nil || got != nil {
			t.Fatalf("GetHardwareID() = (%v, %v), want (nil, nil)", got, err)
		}
	})

	t.Run("bytesReturned exceeding buf grows safely without panic", func(t *testing.T) {
		saveAndResetInterceptionGlobals(t)
		procGetHardwareID = 0x1234
		payload := encodeUTF16LEMultiSZ(`HID\VID_046D&PID_C31C`)
		var seenBufLens []int
		hardwareIDSyscall = func(proc uintptr, ctx Context, dev Device, buf []byte) uint32 {
			seenBufLens = append(seenBufLens, len(buf))
			if len(seenBufLens) == 1 {
				// Return larger count than initial buf without touching buf past len.
				return 6000
			}
			copy(buf, payload)
			return uint32(len(payload))
		}

		got, err := GetHardwareID(1, 2)
		if err != nil {
			t.Fatalf("GetHardwareID() error = %v", err)
		}
		if len(seenBufLens) != 2 || seenBufLens[0] != initialHardwareIDBufBytes || seenBufLens[1] < 6000 {
			t.Fatalf("unexpected retry buffer sizes: %v", seenBufLens)
		}
		if !reflect.DeepEqual(got, []string{`HID\VID_046D&PID_C31C`}) {
			t.Fatalf("got %#v, want Logitech USB ID", got)
		}
	})

	t.Run("bytesReturned exceeding max cap errors cleanly without panic", func(t *testing.T) {
		saveAndResetInterceptionGlobals(t)
		procGetHardwareID = 0x1234
		hardwareIDSyscall = func(proc uintptr, ctx Context, dev Device, buf []byte) uint32 {
			return 99999
		}
		if _, err := GetHardwareID(1, 2); err == nil {
			t.Fatal("expected error when bytesReturned exceeds max cap")
		}
	})

	t.Run("truncated buffer hits cap and fails explicitly", func(t *testing.T) {
		saveAndResetInterceptionGlobals(t)
		procGetHardwareID = 0x1234
		var calls int
		hardwareIDSyscall = func(proc uintptr, ctx Context, dev Device, buf []byte) uint32 {
			calls++
			return uint32(len(buf))
		}
		if _, err := GetHardwareID(1, 2); err == nil {
			t.Fatal("expected truncation error at max buffer cap")
		}
		if calls < 2 {
			t.Fatalf("expected buffer growth retries before error, got %d calls", calls)
		}
	})
}
