package window

import "testing"

func TestDpiAwarenessProcsResolve(t *testing.T) {
	if err := ProcGetDpiAwarenessContextForProcess.Find(); err != nil {
		t.Fatal(err)
	}
	if err := ProcGetThreadDpiAwarenessContext.Find(); err != nil {
		t.Fatal(err)
	}
	if err := ProcGetAwarenessFromDpiAwarenessContext.Find(); err != nil {
		t.Fatal(err)
	}
}
