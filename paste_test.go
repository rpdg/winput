package winput

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/rpdg/winput/hid"
)

func TestDesktopPasteInvalidReceiverPriority(t *testing.T) {
	swaps := 0
	restoreSwap := swapPasteSwap(func(string) (func() error, error) {
		swaps++
		return nil, nil
	})
	defer restoreSwap()
	keys := 0
	restoreKey := swapPasteHotkey(func(Backend, ...Key) error {
		keys++
		return nil
	})
	defer restoreKey()

	var zero DesktopInput
	for _, d := range []*DesktopInput{nil, &zero} {
		for _, text := range []string{"", "not empty"} {
			err := d.PasteByClipboard(text)
			if !errors.Is(err, ErrInputInvalid) {
				t.Fatalf("PasteByClipboard(%q) = %v, want ErrInputInvalid", text, err)
			}
		}
	}
	if swaps != 0 || keys != 0 {
		t.Fatalf("invalid receivers touched swap/key: %d/%d", swaps, keys)
	}
}

func TestDesktopPasteValidEmptyTextDoesNotLockOrTouchClipboard(t *testing.T) {
	in := &Input{backend: BackendMessage, valid: true}
	desk, err := in.Desktop()
	if err != nil {
		t.Fatal(err)
	}
	swaps := 0
	restoreSwap := swapPasteSwap(func(string) (func() error, error) {
		swaps++
		return nil, nil
	})
	defer restoreSwap()
	keys := 0
	restoreKey := swapPasteHotkey(func(Backend, ...Key) error {
		keys++
		return nil
	})
	defer restoreKey()

	// Hold the lock: an implementation that acquired it before validating the
	// text would block here instead of returning immediately.
	inputMutex.Lock()
	done := make(chan error, 1)
	go func() { done <- desk.PasteByClipboard("") }()
	select {
	case err := <-done:
		inputMutex.Unlock()
		if !errors.Is(err, ErrEmptyPasteText) {
			t.Fatalf("empty facade paste = %v, want ErrEmptyPasteText", err)
		}
	case <-time.After(2 * time.Second):
		inputMutex.Unlock()
		t.Fatal("empty facade paste waited for inputMutex")
	}
	if swaps != 0 || keys != 0 {
		t.Fatalf("empty facade paste touched swap/key: %d/%d", swaps, keys)
	}

	if err := PasteByClipboard(""); !errors.Is(err, ErrEmptyPasteText) {
		t.Fatalf("empty package paste = %v, want ErrEmptyPasteText", err)
	}
	if swaps != 0 || keys != 0 {
		t.Fatalf("empty package paste touched swap/key: %d/%d", swaps, keys)
	}
}

func TestPasteByClipboardSequenceUsesFixedBackend(t *testing.T) {
	prev := getBackend()
	defer setGlobalBackend(prev)

	cases := []struct {
		name   string
		fixed  Backend
		global Backend
	}{
		{"fixed HID with global Message", BackendHID, BackendMessage},
		{"fixed Message with global HID", BackendMessage, BackendHID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setGlobalBackend(tc.global)
			inits := 0
			restoreInit := swapInit(func() error {
				inits++
				return errors.New("fixed paste must not initialize HID")
			})
			defer restoreInit()

			in := &Input{backend: tc.fixed, valid: true}
			desk, err := in.Desktop()
			if err != nil {
				t.Fatal(err)
			}
			var order []string
			restoreSwap := swapPasteSwap(func(text string) (func() error, error) {
				requirePasteLock(t)
				order = append(order, "swap")
				return func() error {
					requirePasteLock(t)
					order = append(order, "restore")
					return nil
				}, nil
			})
			defer restoreSwap()
			restoreKey := swapPasteHotkey(func(cb Backend, keys ...Key) error {
				requirePasteLock(t)
				if cb != tc.fixed {
					t.Fatalf("Ctrl+V backend = %v, want %v", cb, tc.fixed)
				}
				if !reflect.DeepEqual(keys, []Key{KeyCtrl, KeyV}) {
					t.Fatalf("hotkey keys = %v", keys)
				}
				order = append(order, "key")
				return nil
			})
			defer restoreKey()

			if err := desk.PasteByClipboard("hello"); err != nil {
				t.Fatalf("paste = %v", err)
			}
			if !reflect.DeepEqual(order, []string{"swap", "key", "restore"}) {
				t.Fatalf("order = %#v", order)
			}
			if inits != 0 {
				t.Fatalf("fixed paste initialized HID %d times", inits)
			}
			assertPasteLockFree(t)
		})
	}
}

func TestPackagePasteUsesResolvedGlobalBackend(t *testing.T) {
	prev := getBackend()
	defer setGlobalBackend(prev)

	var order []string
	restoreSwap := swapPasteSwap(func(string) (func() error, error) {
		requirePasteLock(t)
		order = append(order, "swap")
		return func() error {
			requirePasteLock(t)
			order = append(order, "restore")
			return nil
		}, nil
	})
	defer restoreSwap()

	var got []Backend
	restoreKey := swapPasteHotkey(func(cb Backend, keys ...Key) error {
		requirePasteLock(t)
		got = append(got, cb)
		if !reflect.DeepEqual(keys, []Key{KeyCtrl, KeyV}) {
			t.Fatalf("hotkey keys = %v", keys)
		}
		order = append(order, "key")
		return nil
	})
	defer restoreKey()

	setGlobalBackend(BackendMessage)
	if err := PasteByClipboard("hello"); err != nil {
		t.Fatalf("message paste = %v", err)
	}

	inits := 0
	restoreInit := swapInit(func() error {
		inits++
		return nil
	})
	defer restoreInit()
	setGlobalBackend(BackendHID)
	if err := PasteByClipboard("hello"); err != nil {
		t.Fatalf("HID paste = %v", err)
	}
	if !reflect.DeepEqual(got, []Backend{BackendMessage, BackendHID}) {
		t.Fatalf("resolved backends = %v", got)
	}
	if inits != 1 {
		t.Fatalf("HID global resolution initialized %d times, want 1", inits)
	}
	if !reflect.DeepEqual(order, []string{"swap", "key", "restore", "swap", "key", "restore"}) {
		t.Fatalf("order = %#v", order)
	}
}

func TestPackagePasteGlobalResolutionFailureLeavesClipboardAlone(t *testing.T) {
	prev := getBackend()
	defer setGlobalBackend(prev)
	setGlobalBackend(BackendHID)
	restoreInit := swapInit(func() error { return hid.ErrDriverNotInstalled })
	defer restoreInit()

	swaps := 0
	restoreSwap := swapPasteSwap(func(string) (func() error, error) {
		swaps++
		return nil, nil
	})
	defer restoreSwap()
	keys := 0
	restoreKey := swapPasteHotkey(func(Backend, ...Key) error {
		keys++
		return nil
	})
	defer restoreKey()

	err := PasteByClipboard("hello")
	if !errors.Is(err, ErrDriverNotInstalled) {
		t.Fatalf("global resolution error = %v, want ErrDriverNotInstalled", err)
	}
	if swaps != 0 || keys != 0 {
		t.Fatalf("global resolution failure touched swap/key: %d/%d", swaps, keys)
	}
}

func TestFixedPasteDoesNotPrecheckGlobalHID(t *testing.T) {
	prev := getBackend()
	defer setGlobalBackend(prev)
	setGlobalBackend(BackendHID)

	inits := 0
	restoreInit := swapInit(func() error {
		inits++
		return errors.New("global HID initialization must not run")
	})
	defer restoreInit()

	in := &Input{backend: BackendMessage, valid: true}
	desk, err := in.Desktop()
	if err != nil {
		t.Fatal(err)
	}
	swaps := 0
	restoreSwap := swapPasteSwap(func(string) (func() error, error) {
		swaps++
		return func() error { return nil }, nil
	})
	defer restoreSwap()
	var keyBackend Backend
	restoreKey := swapPasteHotkey(func(cb Backend, _ ...Key) error {
		keyBackend = cb
		return nil
	})
	defer restoreKey()

	if err := desk.PasteByClipboard("hello"); err != nil {
		t.Fatalf("fixed paste = %v", err)
	}
	if swaps != 1 || keyBackend != BackendMessage {
		t.Fatalf("swaps/backend = %d/%v", swaps, keyBackend)
	}
	if inits != 0 {
		t.Fatalf("fixed paste read global HID and initialized %d times", inits)
	}
}

func TestPasteByClipboardErrorRecovery(t *testing.T) {
	swapErr := errors.New("swap failed")
	restoreErr := errors.New("restore failed")
	cases := []struct {
		name        string
		swapErr     error
		withRestore bool
		keyErr      error
		restoreErr  error
		wantKeys    int
		wantRestore int
		wantOrder   []string
		wantErrors  []error
	}{
		{
			name:        "swap failure without restore",
			swapErr:     swapErr,
			withRestore: false,
			wantOrder:   []string{"swap"},
			wantErrors:  []error{swapErr},
		},
		{
			name:        "swap failure still restores",
			swapErr:     swapErr,
			withRestore: true,
			restoreErr:  restoreErr,
			wantRestore: 1,
			wantOrder:   []string{"swap", "restore"},
			wantErrors:  []error{swapErr, restoreErr},
		},
		{
			name:        "key failure still restores",
			withRestore: true,
			keyErr:      ErrSendInputFailed,
			restoreErr:  ErrClipboardFailed,
			wantKeys:    1,
			wantRestore: 1,
			wantOrder:   []string{"swap", "key", "restore"},
			wantErrors:  []error{ErrSendInputFailed, ErrClipboardFailed},
		},
		{
			name:        "restore failure after successful key",
			withRestore: true,
			restoreErr:  restoreErr,
			wantKeys:    1,
			wantRestore: 1,
			wantOrder:   []string{"swap", "key", "restore"},
			wantErrors:  []error{restoreErr},
		},
		{
			name:        "all success",
			withRestore: true,
			wantKeys:    1,
			wantRestore: 1,
			wantOrder:   []string{"swap", "key", "restore"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := &Input{backend: BackendMessage, valid: true}
			desk, err := in.Desktop()
			if err != nil {
				t.Fatal(err)
			}
			var order []string
			restoreSwap := swapPasteSwap(func(string) (func() error, error) {
				order = append(order, "swap")
				var restore func() error
				if tc.withRestore {
					restore = func() error {
						order = append(order, "restore")
						return tc.restoreErr
					}
				}
				return restore, tc.swapErr
			})
			defer restoreSwap()
			keys, restores := 0, 0
			restoreKey := swapPasteHotkey(func(Backend, ...Key) error {
				keys++
				order = append(order, "key")
				return tc.keyErr
			})
			defer restoreKey()

			err = desk.PasteByClipboard("hello")
			for _, want := range tc.wantErrors {
				if !errors.Is(err, want) {
					t.Errorf("error %v does not contain %v", err, want)
				}
			}
			if len(tc.wantErrors) == 0 && err != nil {
				t.Fatalf("paste = %v, want nil", err)
			}
			restores = countRestore(order)
			if keys != tc.wantKeys || restores != tc.wantRestore {
				t.Fatalf("keys/restores = %d/%d, want %d/%d", keys, restores, tc.wantKeys, tc.wantRestore)
			}
			if !reflect.DeepEqual(order, tc.wantOrder) {
				t.Fatalf("order = %#v, want %#v", order, tc.wantOrder)
			}
		})
	}
}

func TestFixedHIDRuntimeErrorRetainsTypeAndRestores(t *testing.T) {
	prev := getBackend()
	defer setGlobalBackend(prev)
	setGlobalBackend(BackendMessage)
	restoreInit := swapInit(func() error { return errors.New("not called by fixed path") })
	defer restoreInit()

	runtimeErr := errors.New("HID runtime failure")
	in := &Input{backend: BackendHID, valid: true}
	desk, err := in.Desktop()
	if err != nil {
		t.Fatal(err)
	}
	restored := 0
	restoreSwap := swapPasteSwap(func(string) (func() error, error) {
		return func() error { restored++; return nil }, nil
	})
	defer restoreSwap()
	restoreKey := swapPasteHotkey(func(Backend, ...Key) error { return runtimeErr })
	defer restoreKey()

	err = desk.PasteByClipboard("hello")
	if !errors.Is(err, runtimeErr) {
		t.Fatalf("paste error = %v, want original runtime error", err)
	}
	if errors.Is(err, ErrSendInputFailed) {
		t.Fatal("HID runtime error was converted to ErrSendInputFailed")
	}
	if restored != 1 {
		t.Fatalf("restore calls = %d, want 1", restored)
	}
}

func swapPasteSwap(fn func(string) (func() error, error)) func() {
	prev := swapUnicodeText
	swapUnicodeText = fn
	return func() { swapUnicodeText = prev }
}

func swapPasteHotkey(fn func(Backend, ...Key) error) func() {
	prev := pasteHotkey
	pasteHotkey = fn
	return func() { pasteHotkey = prev }
}

func requirePasteLock(t *testing.T) {
	t.Helper()
	if inputMutex.TryLock() {
		inputMutex.Unlock()
		t.Error("inputMutex is not held")
	}
}

func assertPasteLockFree(t *testing.T) {
	t.Helper()
	if !inputMutex.TryLock() {
		t.Fatal("inputMutex was left locked")
	}
	inputMutex.Unlock()
}

func countRestore(order []string) int {
	n := 0
	for _, step := range order {
		if step == "restore" {
			n++
		}
	}
	return n
}

func setGlobalBackend(b Backend) {
	backendMutex.Lock()
	currentBackend = b
	backendMutex.Unlock()
}
