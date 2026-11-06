package winput

import (
	"errors"

	"github.com/rpdg/winput/hid"
	"github.com/rpdg/winput/keyboard"
	"github.com/rpdg/winput/mouse"
	"github.com/rpdg/winput/window"
)

var (
	// ErrWindowNotFound implies the target window could not be located by Title, Class, or PID.
	ErrWindowNotFound = errors.New("window not found")

	// ErrWindowGone implies the window handle is no longer valid.
	ErrWindowGone = errors.New("window is gone or invalid")

	// ErrWindowNotVisible implies the window is hidden or minimized.
	ErrWindowNotVisible = errors.New("window is not visible")

	// ErrUnsupportedKey implies the character cannot be mapped to a key.
	ErrUnsupportedKey = keyboard.ErrUnsupportedKey

	// ErrBackendUnavailable implies the selected backend (e.g. HID) failed to initialize.
	ErrBackendUnavailable = errors.New("input backend unavailable")

	// ErrDriverNotInstalled specific to BackendHID, implies the Interception driver is missing or not accessible.
	ErrDriverNotInstalled = errors.New("interception driver not installed or accessible")

	// ErrDLLLoadFailed implies interception.dll could not be loaded.
	ErrDLLLoadFailed = errors.New("failed to load interception library")

	// ErrPermissionDenied implies the operation failed due to system privilege restrictions (e.g. UIPI).
	ErrPermissionDenied = errors.New("permission denied")

	// ErrPostMessageFailed implies the PostMessageW call returned 0.
	ErrPostMessageFailed = window.ErrPostMessageFailed

	// ErrReadTextFailed implies the library could not read text from the target window/control.
	ErrReadTextFailed = window.ErrReadTextFailed

	// ErrMoveNotConverged means the HID cursor did not reach the requested
	// tolerance. winput.ErrMoveNotConverged aliases hid.ErrMoveNotConverged.
	ErrMoveNotConverged = hid.ErrMoveNotConverged

	// ErrOutsideVirtualDesktop means the target point is outside the virtual desktop.
	ErrOutsideVirtualDesktop = hid.ErrOutsideVirtualDesktop

	// ErrSendInputFailed means SendInput inserted zero events.
	ErrSendInputFailed = errors.New("SendInput inserted no events")

	// ErrSendInputPartial means SendInput inserted only the key-down event,
	// and the compensating key-up was inserted successfully.
	ErrSendInputPartial = errors.New("SendInput partially inserted and was repaired")

	// ErrSendInputStuck means SendInput inserted only the key-down event,
	// and the compensating key-up could not be inserted.
	ErrSendInputStuck = errors.New("SendInput partially inserted and could not be repaired")

	// ErrDirectedScrollUnsupported means the HID backend cannot direct a wheel
	// event to a specific window.
	ErrDirectedScrollUnsupported = errors.New("HID backend does not support directed scrolling")

	// ErrInvalidMouseButton means the MouseButton value is not one of the exported identities.
	ErrInvalidMouseButton = errors.New("invalid mouse button")

	// ErrInvalidMouseCoordinate means a mouse message coordinate does not fit
	// in the signed 16-bit halves of lParam. The message is not sent.
	ErrInvalidMouseCoordinate = mouse.ErrInvalidMouseCoordinate

	// ErrInvalidScrollDelta means a directed scroll delta does not fit in int16.
	// Window.Scroll on BackendMessage returns this value, which aliases
	// mouse.ErrInvalidScrollDelta. hid.ErrInvalidScrollDelta is a separate
	// sentinel; hid does not import mouse.
	ErrInvalidScrollDelta = mouse.ErrInvalidScrollDelta

	// ErrInvalidScrollSteps means ScrollWheel steps are zero or outside [-273, 273].
	ErrInvalidScrollSteps = errors.New("scroll steps must be non-zero and within [-273, 273]")

	// ErrInvalidPressDuration means PressFor was given less than 10ms.
	ErrInvalidPressDuration = errors.New("press duration must be at least 10ms")

	// ErrInvalidInterval means a variable interval argument to Press or Type
	// was negative or there was more than one argument. It is defined in
	// keyboard because keyboard.TypeWithInterval returns it too. It is
	// deliberately not the same sentinel as ErrInvalidPressDuration: an
	// interval of 0 is legal, and a second interval argument is an error.
	ErrInvalidInterval = keyboard.ErrInvalidInterval

	// ErrEmptyPasteText means PasteByClipboard was called with an empty string.
	// The clipboard is not opened and no key is sent.
	ErrEmptyPasteText = errors.New("paste text is empty")

	// ErrClipboardFailed means a clipboard open, read, write, or restore
	// failed. It aliases window.ErrClipboardFailed; the error text never
	// includes clipboard contents.
	ErrClipboardFailed = window.ErrClipboardFailed
)
