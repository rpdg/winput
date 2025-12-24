package winput

import "errors"

var (
	// ErrWindowNotFound implies the target window could not be located by Title, Class, or PID.
	ErrWindowNotFound = errors.New("window not found")

	// ErrWindowGone implies the window handle is no longer valid (window closed).
	ErrWindowGone = errors.New("window is gone or invalid")

	// ErrWindowNotVisible implies the window is hidden or minimized.
	ErrWindowNotVisible = errors.New("window is not visible")

	// ErrUnsupportedKey implies the character or key code cannot be mapped to a valid input event.
	ErrUnsupportedKey = errors.New("unsupported key or character")

	// ErrBackendUnavailable implies the selected backend (e.g. HID) failed to initialize.
	ErrBackendUnavailable = errors.New("input backend unavailable")

	// ErrDriverNotInstalled specific to BackendHID, implies the Interception driver is missing or not accessible.
	ErrDriverNotInstalled = errors.New("interception driver not installed or accessible")

	// ErrDLLLoadFailed specific to BackendHID, implies interception.dll could not be loaded.
	ErrDLLLoadFailed = errors.New("failed to load interception library")

	// ErrPermissionDenied implies the operation failed due to system privilege restrictions (e.g. UIPI).
	ErrPermissionDenied = errors.New("permission denied")
)
