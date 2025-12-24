package winput

import "errors"

var (
	ErrWindowNotFound     = errors.New("window not found")
	ErrUnsupportedKey     = errors.New("unsupported key")
	ErrBackendUnavailable = errors.New("backend unavailable")
	ErrPermissionDenied   = errors.New("permission denied")
)
