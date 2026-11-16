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

var hidButtonStates = map[MouseButton]struct{ down, up uint16 }{
	MouseLeft: {1, 2}, MouseRight: {4, 8}, MouseMiddle: {16, 32},
}

func (interceptionBackend) MouseButton(button MouseButton, down bool) error {
	states, ok := hidButtonStates[button]
	if !ok {
		return ErrInvalidMouseButton
	}
	state := states.up
	if down {
		state = states.down
	}
	return hid.Button(state)
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
