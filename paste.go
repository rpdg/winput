package winput

import (
	"errors"

	"github.com/rpdg/winput/window"
)

// swapUnicodeText is window.SwapUnicodeText. Tests replace it to exercise the
// swap/restore error paths without touching the real clipboard.
var swapUnicodeText = window.SwapUnicodeText

// pasteHotkey sends Ctrl+V with the already-resolved backend. It is the only
// key path used by pasteByClipboard and must run while inputMutex is held.
// Tests replace it to record the backend and inject key failures.
var pasteHotkey = pressHotkeyWith

// validatePasteText rejects empty paste text before the clipboard is opened or
// the input mutex is taken. Whitespace-only text is legal paste content.
func validatePasteText(text string) error {
	if text == "" {
		return ErrEmptyPasteText
	}
	return nil
}

// pasteByClipboard runs the shared paste path. fixed == nil is the package-level
// compatibility entry and follows the process-wide global backend. A non-nil
// fixed backend is used as-is: it does not read currentBackend, initialize HID,
// or run a wrapper-level availability check. The text and backend are resolved
// before the clipboard is touched; a global resolution failure leaves the
// clipboard unchanged. A fixed HID runtime failure can still happen after the
// swap because no extra pre-check is performed here.
//
// The swap, Ctrl+V, and restore all run under one inputMutex section. That lock
// only serializes in-process callers that take it; it is not a cross-process
// clipboard lock, desktop lease, or task transaction. Restore is attempted once
// whenever SwapUnicodeText returned a non-nil function, including when the swap
// or the key failed. errors.Join preserves errors.Is for each stage.
func pasteByClipboard(text string, fixed *Backend) error {
	if err := validatePasteText(text); err != nil {
		return err
	}

	inputMutex.Lock()
	defer inputMutex.Unlock()
	cb, err := acquireBackend(fixed)
	if err != nil {
		return err
	}

	restore, swapErr := swapUnicodeText(text)
	var keyErr error
	if swapErr == nil {
		// Only a clean write reaches the key. pasteHotkey takes no lock, so it
		// cannot deadlock against the held inputMutex.
		keyErr = pasteHotkey(cb, KeyCtrl, KeyV)
	}
	var restoreErr error
	if restore != nil {
		restoreErr = restore()
	}
	return errors.Join(swapErr, keyErr, restoreErr)
}

// PasteByClipboard pastes text into the current foreground window using
// Ctrl+V. It takes no HWND: the keystrokes go to whatever window is in the
// foreground when the call runs, so a different foreground than the caller
// expects will receive the text.
//
// This package-level function keeps the process-wide global-default backend
// behavior. The backend is resolved for this call after inputMutex is taken;
// the resolved value is reused for the Ctrl+V. The global default is not an
// atomic snapshot against a concurrent SetBackend, and this function does not
// promise call-level ordering with SetBackend. Use
// DesktopInput.PasteByClipboard for an executor with a fixed backend.
//
// A nil return does NOT mean the foreground program has received the new
// text. It only means the Ctrl+V events were queued and the previous Unicode
// clipboard text was restored without error. SendInput returns once the events
// are queued (it does not wait for the foreground to process the paste), and
// the restore runs right after the combination key; there is no
// synchronization point that proves the foreground read the clipboard before
// it was restored. A non-nil return does not prove the keys were not inserted
// either: the key can have succeeded while the restore failed, in which case
// errors.Is can recognize both ErrClipboardFailed and ErrSendInputFailed on
// the same error. When the swap provides a restore function, restore is
// attempted once even if the swap or Ctrl+V failed; the returned error joins
// the swap, key, and restore errors. To know whether a control accepted the
// text, the caller must observe that control separately.
//
// The clipboard is temporarily replaced and only CF_UNICODETEXT is put back.
// This is not a full clipboard backup and restore. If the system had no Unicode
// format (including when the clipboard only held an image, or a CF_TEXT that
// the system could convert), a bare empty Unicode buffer is restored and the
// original non-text formats (images, HTML, file lists, custom formats) do not
// return. Even when Unicode text was present, other formats are not restored.
// New text has a lone "\n" normalized to "\r\n" before it is written; restored
// text is written byte-for-byte as it was read. Empty text fails with
// ErrEmptyPasteText before the clipboard is opened and before any key is sent;
// whitespace-only text is legal. PasteByClipboard is not a replacement for
// Type; controls that reject paste (for example password fields) will not
// succeed.
//
// inputMutex only coordinates in-process callers that follow it. It does not
// provide cross-process clipboard exclusivity, a desktop lease, or isolation
// from other foreground windows. Event queueing does not prove consumption.
func PasteByClipboard(text string) error {
	return pasteByClipboard(text, nil)
}
