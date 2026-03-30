package rodx

import (
	"errors"
	"testing"
)

func TestInjectRemoteDebuggingArg(t *testing.T) {
	args, err := injectRemoteDebuggingArg([]string{"--foo=bar"}, 9222, "")
	if err != nil {
		t.Fatalf("injectRemoteDebuggingArg failed: %v", err)
	}

	found := false
	for _, arg := range args {
		if arg == "--remote-debugging-port=9222" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing remote debugging arg in %v", args)
	}
}

func TestInjectRemoteDebuggingArgConflict(t *testing.T) {
	_, err := injectRemoteDebuggingArg([]string{"--remote-debugging-port=9222"}, 9333, "")
	if !errors.Is(err, ErrDebugPortConflict) {
		t.Fatalf("expected ErrDebugPortConflict, got %v", err)
	}
}

func TestNormalizeDebugURL(t *testing.T) {
	got, err := normalizeDebugURL("", 9222)
	if err != nil {
		t.Fatalf("normalizeDebugURL failed: %v", err)
	}
	if got != "http://127.0.0.1:9222" {
		t.Fatalf("unexpected debug url: %q", got)
	}
}
