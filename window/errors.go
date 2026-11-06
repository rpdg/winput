package window

import "errors"

var ErrPostMessageFailed = errors.New("PostMessageW failed")

// ErrInvalidLookupString means a title or class contains an embedded NUL.
// FindWindowW treats a NULL pointer as a wildcard, so the lookup is refused
// before that call.
var ErrInvalidLookupString = errors.New("invalid window lookup string")

// ErrClipboardFailed means a clipboard open, read, write, or restore failed.
// The error text never includes clipboard contents or LazyProc.Find text.
var ErrClipboardFailed = errors.New("clipboard operation failed")
