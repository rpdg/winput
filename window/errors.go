package window

import "errors"

var ErrPostMessageFailed = errors.New("PostMessageW failed")

// ErrClipboardFailed means a clipboard open, read, write, or restore failed.
// The error text never includes clipboard contents or LazyProc.Find text.
var ErrClipboardFailed = errors.New("clipboard operation failed")
