package winput

// inputBackend contains only device input operations. Message input keeps its
// existing HWND and system API paths in the root package.
type inputBackend interface {
	KeyDown(Key) error
	KeyUp(Key) error
	MoveTo(x, y int32) error
	MoveToExact(x, y int32) error
	MoveRelative(dx, dy int32) error
	MouseButton(MouseButton, bool) error
	ClickAt(MouseButton, int32, int32) error
	DoubleClickAt(int32, int32) error
	Scroll(delta int32) error
}

// backendSelection is copied as one immutable value, including for fixed Input.
type backendSelection struct {
	kind   Backend
	device inputBackend
}

// prepareSelection performs initialization without either root-package lock.
// Unknown kinds remain Message selections; NewInput validates separately.
func prepareSelection(kind Backend) (backendSelection, error) {
	sel := backendSelection{kind: kind}
	if kind == BackendHID {
		if err := ensureHID(); err != nil {
			return backendSelection{}, err
		}
		sel.device = interceptionBackend{}
	}
	return sel, nil
}
