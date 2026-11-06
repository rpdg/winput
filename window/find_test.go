package window

import (
	"strings"
	"syscall"
	"testing"
)

func TestFindRejectsEmbeddedNUL(t *testing.T) {
	cases := []struct {
		name string
		call func() error
	}{
		{"title", func() error { _, err := FindByTitle("a\x00b"); return err }},
		{"class", func() error { _, err := FindByClass("\x00"); return err }},
		{"child", func() error { _, err := FindChildByClass(0, "Edit\x00"); return err }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if err == nil {
				t.Fatal("embedded NUL was treated as a match")
			}
			if !strings.Contains(err.Error(), "invalid") {
				t.Fatalf("error = %v, want an invalid-input error", err)
			}
		})
	}
}

func TestFindByTitleEmptyIsNotRejectedAsInvalid(t *testing.T) {
	_, err := FindByTitle("")
	if err != nil && strings.Contains(err.Error(), "invalid window title") {
		t.Fatalf("empty title rejected before lookup: %v", err)
	}
}

func TestMatchingPIDsKeepsEveryExe(t *testing.T) {
	entries := []processEntry{
		{pid: 10, exe: "chrome.exe"},
		{pid: 20, exe: "notepad.exe"},
		{pid: 30, exe: "Notepad.EXE"},
		{pid: 40, exe: "explorer.exe"},
	}
	got := matchingPIDs(entries, "notepad")
	if len(got) != 2 || got[0] != 20 || got[1] != 30 {
		t.Fatalf("matchingPIDs(notepad) = %v, want [20 30]", got)
	}
	got = matchingPIDs(entries, "chrome.exe")
	if len(got) != 1 || got[0] != 10 {
		t.Fatalf("matchingPIDs(chrome.exe) = %v, want [10]", got)
	}
	if len(matchingPIDs(entries, "missing")) != 0 {
		t.Fatal("missing exe returned a pid")
	}
}

func TestUTF16PtrFromStringNUL(t *testing.T) {
	ptr, err := syscall.UTF16PtrFromString("a\x00b")
	if ptr != nil || err == nil {
		t.Fatalf("UTF16PtrFromString(NUL) = %v, %v, want nil and error", ptr, err)
	}
}
