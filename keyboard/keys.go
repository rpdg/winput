package keyboard

// Key represents a hardware scan code.
//
// The low 8 bits hold the Set 1 scan code. Keys that require the E0 prefix
// also set the high byte to 0xE0. Use Decode rather than interpreting the
// whole value as a scan code.
type Key uint16

const (
	KeyEsc            Key = 0x01
	Key1              Key = 0x02
	Key2              Key = 0x03
	Key3              Key = 0x04
	Key4              Key = 0x05
	Key5              Key = 0x06
	Key6              Key = 0x07
	Key7              Key = 0x08
	Key8              Key = 0x09
	Key9              Key = 0x0A
	Key0              Key = 0x0B
	KeyMinus          Key = 0x0C
	KeyEqual          Key = 0x0D
	KeyBkSp           Key = 0x0E
	KeyTab            Key = 0x0F
	KeyQ              Key = 0x10
	KeyW              Key = 0x11
	KeyE              Key = 0x12
	KeyR              Key = 0x13
	KeyT              Key = 0x14
	KeyY              Key = 0x15
	KeyU              Key = 0x16
	KeyI              Key = 0x17
	KeyO              Key = 0x18
	KeyP              Key = 0x19
	KeyLBr            Key = 0x1A
	KeyRBr            Key = 0x1B
	KeyEnter          Key = 0x1C
	KeyLeftCtrl       Key = 0x1D
	KeyCtrl           Key = KeyLeftCtrl
	KeyA              Key = 0x1E
	KeyS              Key = 0x1F
	KeyD              Key = 0x20
	KeyF              Key = 0x21
	KeyG              Key = 0x22
	KeyH              Key = 0x23
	KeyJ              Key = 0x24
	KeyK              Key = 0x25
	KeyL              Key = 0x26
	KeySemi           Key = 0x27
	KeyQuot           Key = 0x28
	KeyTick           Key = 0x29
	KeyLeftShift      Key = 0x2A
	KeyShift          Key = KeyLeftShift
	KeyBackslash      Key = 0x2B
	KeyZ              Key = 0x2C
	KeyX              Key = 0x2D
	KeyC              Key = 0x2E
	KeyV              Key = 0x2F
	KeyB              Key = 0x30
	KeyN              Key = 0x31
	KeyM              Key = 0x32
	KeyComma          Key = 0x33
	KeyDot            Key = 0x34
	KeySlash          Key = 0x35
	KeyRightShift     Key = 0x36
	KeyNumpadMultiply Key = 0x37
	KeyLeftAlt        Key = 0x38
	KeyAlt            Key = KeyLeftAlt
	KeySpace          Key = 0x39
	KeyCaps           Key = 0x3A
	KeyF1             Key = 0x3B
	KeyF2             Key = 0x3C
	KeyF3             Key = 0x3D
	KeyF4             Key = 0x3E
	KeyF5             Key = 0x3F
	KeyF6             Key = 0x40
	KeyF7             Key = 0x41
	KeyF8             Key = 0x42
	KeyF9             Key = 0x43
	KeyF10            Key = 0x44
	KeyNumLock        Key = 0x45
	KeyScroll         Key = 0x46
	KeyNumpad7        Key = 0x47
	KeyNumpad8        Key = 0x48
	KeyNumpad9        Key = 0x49
	KeyNumpadSubtract Key = 0x4A
	KeyNumpad4        Key = 0x4B
	KeyNumpad5        Key = 0x4C
	KeyNumpad6        Key = 0x4D
	KeyNumpadAdd      Key = 0x4E
	KeyNumpad1        Key = 0x4F
	KeyNumpad2        Key = 0x50
	KeyNumpad3        Key = 0x51
	KeyNumpad0        Key = 0x52
	KeyNumpadDecimal  Key = 0x53
	KeyF11            Key = 0x57
	KeyF12            Key = 0x58

	// Extended Keys. These carry the E0 prefix in the high byte.
	KeyHome      Key = 0xE047
	KeyArrowUp   Key = 0xE048
	KeyPageUp    Key = 0xE049
	KeyLeft      Key = 0xE04B
	KeyRight     Key = 0xE04D
	KeyEnd       Key = 0xE04F
	KeyArrowDown Key = 0xE050
	KeyPageDown  Key = 0xE051
	KeyInsert    Key = 0xE052
	KeyDelete    Key = 0xE053

	KeyRightCtrl    Key = 0xE01D
	KeyRightAlt     Key = 0xE038
	KeyNumpadDivide Key = 0xE035
	KeyDivide       Key = KeyNumpadDivide
	KeyNumpadEnter  Key = 0xE01C
	KeyLWin         Key = 0xE05B
	KeyRWin         Key = 0xE05C
)

// Decode splits a Key into its Set 1 scan code and E0 flag.
// A zero scan code or a high byte other than 0x00/0xE0 is rejected.
func (k Key) Decode() (scan uint8, extended bool, ok bool) {
	switch uint16(k) >> 8 {
	case 0x00:
	case 0xE0:
		extended = true
	default:
		return 0, false, false
	}

	scan = uint8(k)
	if scan == 0 {
		return 0, false, false
	}
	return scan, extended, true
}

// KeyDef represents a key definition mapping a rune to a scan code.
type KeyDef struct {
	Code    Key
	Shifted bool
}

var runeMap = map[rune]KeyDef{
	'a': {KeyA, false}, 'A': {KeyA, true},
	'b': {KeyB, false}, 'B': {KeyB, true},
	'c': {KeyC, false}, 'C': {KeyC, true},
	'd': {KeyD, false}, 'D': {KeyD, true},
	'e': {KeyE, false}, 'E': {KeyE, true},
	'f': {KeyF, false}, 'F': {KeyF, true},
	'g': {KeyG, false}, 'G': {KeyG, true},
	'h': {KeyH, false}, 'H': {KeyH, true},
	'i': {KeyI, false}, 'I': {KeyI, true},
	'j': {KeyJ, false}, 'J': {KeyJ, true},
	'k': {KeyK, false}, 'K': {KeyK, true},
	'l': {KeyL, false}, 'L': {KeyL, true},
	'm': {KeyM, false}, 'M': {KeyM, true},
	'n': {KeyN, false}, 'N': {KeyN, true},
	'o': {KeyO, false}, 'O': {KeyO, true},
	'p': {KeyP, false}, 'P': {KeyP, true},
	'q': {KeyQ, false}, 'Q': {KeyQ, true},
	'r': {KeyR, false}, 'R': {KeyR, true},
	's': {KeyS, false}, 'S': {KeyS, true},
	't': {KeyT, false}, 'T': {KeyT, true},
	'u': {KeyU, false}, 'U': {KeyU, true},
	'v': {KeyV, false}, 'V': {KeyV, true},
	'w': {KeyW, false}, 'W': {KeyW, true},
	'x': {KeyX, false}, 'X': {KeyX, true},
	'y': {KeyY, false}, 'Y': {KeyY, true},
	'z': {KeyZ, false}, 'Z': {KeyZ, true},

	'0': {Key0, false}, ')': {Key0, true},
	'1': {Key1, false}, '!': {Key1, true},
	'2': {Key2, false}, '@': {Key2, true},
	'3': {Key3, false}, '#': {Key3, true},
	'4': {Key4, false}, '$': {Key4, true},
	'5': {Key5, false}, '%': {Key5, true},
	'6': {Key6, false}, '^': {Key6, true},
	'7': {Key7, false}, '&': {Key7, true},
	'8': {Key8, false}, '*': {Key8, true},
	'9': {Key9, false}, '(': {Key9, true},

	'`': {KeyTick, false}, '~': {KeyTick, true},
	'-': {KeyMinus, false}, '_': {KeyMinus, true},
	'=': {KeyEqual, false}, '+': {KeyEqual, true},
	'[': {KeyLBr, false}, '{': {KeyLBr, true},
	']': {KeyRBr, false}, '}': {KeyRBr, true},
	'\\': {KeyBackslash, false}, '|': {KeyBackslash, true},
	';': {KeySemi, false}, ':': {KeySemi, true},
	'\'': {KeyQuot, false}, '"': {KeyQuot, true},
	',': {KeyComma, false}, '<': {KeyComma, true},
	'.': {KeyDot, false}, '>': {KeyDot, true},
	'/': {KeySlash, false}, '?': {KeySlash, true},

	' ':  {KeySpace, false},
	'\n': {KeyEnter, false},
	'\t': {KeyTab, false},
}

// LookupKey returns the Scan Code and whether Shift is required.
func LookupKey(r rune) (Key, bool, bool) {
	k, ok := runeMap[r]
	return k.Code, k.Shifted, ok
}
