package winput

import (
	"errors"

	"github.com/rpdg/winput/window"
)

// validatePasteText rejects empty paste text before the clipboard is opened or
// the input mutex is taken. Whitespace-only text is legal paste content.
func validatePasteText(text string) error {
	if text == "" {
		return ErrEmptyPasteText
	}
	return nil
}

// PasteByClipboard pastes text into the current foreground window using
// Ctrl+V. It takes no HWND: the keystrokes go to whatever window is in the
// foreground when the call runs, so a different foreground than the caller
// expects will receive the text.
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
// the same error. To know whether a control accepted the text, the caller must
// observe that control separately.
//
// The clipboard is temporarily replaced and only CF_UNICODETEXT is put back.
// If the system had no Unicode format (including when the clipboard only held
// an image, or a CF_TEXT that the system could convert), a bare empty Unicode
// buffer is restored and the original non-text formats do not return. New text
// has a lone "\n" normalized to "\r\n" before it is written; restored text is
// written byte-for-byte as it was read. Empty text fails with
// ErrEmptyPasteText before the clipboard is opened and before any key is sent.
// PasteByClipboard is not a replacement for Type; controls that reject paste
// (for example password fields) will not succeed.
func PasteByClipboard(text string) error {
	if err := validatePasteText(text); err != nil {
		return err
	}

	inputMutex.Lock()
	defer inputMutex.Unlock()
	if err := checkBackend(); err != nil {
		return err
	}

	restore, swapErr := window.SwapUnicodeText(text)
	var keyErr error
	if swapErr == nil {
		// Only a clean write reaches the key. SwapUnicodeText is the sole
		// caller-visible clipboard path; pressHotkeyLocked takes no lock, so
		// this cannot deadlock against the held inputMutex.
		keyErr = pressHotkeyLocked(KeyCtrl, KeyV)
	}
	var restoreErr error
	if restore != nil {
		restoreErr = restore()
	}
	return errors.Join(swapErr, keyErr, restoreErr)
}
