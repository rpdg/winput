package uia

import (
	"runtime"
	"testing"
	"unsafe"
)

func TestVariantLayout(t *testing.T) {
	var v variant
	if off := unsafe.Offsetof(v.Val); off != 8 {
		t.Fatalf("Val offset = %d, want 8", off)
	}
	v.VT = vtI4
	v.Val = 0x1234
	switch runtime.GOARCH {
	case "386":
		if unsafe.Sizeof(v) != 16 {
			t.Fatalf("sizeof(variant) = %d, want 16", unsafe.Sizeof(v))
		}
		words := (*[4]uintptr)(unsafe.Pointer(&v))
		if uint16(words[0]) != vtI4 {
			t.Fatalf("first word = %#x, want VT_I4 in the low half", words[0])
		}
		// Offset 8 is the third 32-bit word.
		if uint32(words[2]) != 0x1234 {
			t.Fatalf("value word = %#x, want 0x1234", uint32(words[2]))
		}
	case "amd64", "arm64":
		if unsafe.Sizeof(v) != 24 {
			t.Fatalf("sizeof(variant) = %d, want 24", unsafe.Sizeof(v))
		}
	default:
		t.Fatalf("unsupported GOARCH %s", runtime.GOARCH)
	}
}
