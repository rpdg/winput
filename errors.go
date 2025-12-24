package winput

import "errors"

var (
	// ErrWindowNotFound implies the target window could not be located by Title, Class, or PID.
	ErrWindowNotFound = errors.New("window not found")

	// ErrUnsupportedKey implies the character or key code cannot be mapped to a valid input event.
	ErrUnsupportedKey = errors.New("unsupported key or character")

	// ErrBackendUnavailable implies the selected backend (e.g. HID) failed to initialize.
	ErrBackendUnavailable = errors.New("input backend unavailable")

	// ErrDriverNotInstalled specific to BackendHID, implies the Interception driver is missing or not accessible.
	ErrDriverNotInstalled = errors.New("interception driver not installed or accessible")

	// ErrPermissionDenied implies the operation failed due to system privilege restrictions (e.g. UIPI).
	ErrPermissionDenied = errors.New("permission denied")
)
