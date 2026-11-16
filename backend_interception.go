package winput

import "github.com/rpdg/winput/hid"

// interceptionBackend is deliberately stateless. Keep lifecycle, coordinates,
// trajectories, retries and timing in the existing hid operations.
type interceptionBackend struct{}

var _ inputBackend = interceptionBackend{}

func (interceptionBackend) KeyDown(k Key) error           { return hid.KeyDown(uint16(k)) }
func (interceptionBackend) KeyUp(k Key) error             { return hid.KeyUp(uint16(k)) }
func (interceptionBackend) MoveTo(x, y int32) error       { return hid.Move(x, y) }
func (interceptionBackend) MoveToExact(x, y int32) error  { return hid.MoveExact(x, y) }
func (interceptionBackend) MoveRelative(x, y int32) error { return hid.MoveRelative(x, y) }

func hidButtonStates(button MouseButton) (down, up uint16, ok bool) {
	switch button {
	case MouseLeft:
		return 0x001, 0x002, true
	case MouseRight:
		return 0x004, 0x008, true
	case MouseMiddle:
		return 0x010, 0x020, true
	default:
		return 0, 0, false
	}
}

func (interceptionBackend) MouseButton(button MouseButton, down bool) error {
	d, u, ok := hidButtonStates(button)
	if !ok {
		return ErrInvalidMouseButton
	}
	if down {
		return hid.Button(d)
	}
	return hid.Button(u)
}

func (interceptionBackend) ClickAt(button MouseButton, x, y int32) error {
	switch button {
	case MouseLeft:
		return hid.Click(x, y)
	case MouseRight:
		return hid.ClickRight(x, y)
	case MouseMiddle:
		return hid.ClickMiddle(x, y)
	default:
		return ErrInvalidMouseButton
	}
}
func (interceptionBackend) DoubleClickAt(x, y int32) error { return hid.DoubleClick(x, y) }
func (interceptionBackend) Scroll(delta int32) error       { return hid.Scroll(delta) }
