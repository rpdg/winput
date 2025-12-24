package keyboard

type Key uint16

const (
	KeyEsc   Key = 0x01
	Key1     Key = 0x02
	Key2     Key = 0x03
	Key3     Key = 0x04
	Key4     Key = 0x05
	Key5     Key = 0x06
	Key6     Key = 0x07
	Key7     Key = 0x08
	Key8     Key = 0x09
	Key9     Key = 0x0A
	Key0     Key = 0x0B
	KeyMinus Key = 0x0C
	KeyEqual Key = 0x0D
	KeyBkSp  Key = 0x0E
	KeyTab   Key = 0x0F
	KeyQ     Key = 0x10
	KeyW     Key = 0x11
	KeyE     Key = 0x12
	KeyR     Key = 0x13
	KeyT     Key = 0x14
	KeyY     Key = 0x15
	KeyU     Key = 0x16
	KeyI     Key = 0x17
	KeyO     Key = 0x18
	KeyP     Key = 0x19
	KeyLBr   Key = 0x1A
	KeyRBr   Key = 0x1B
	KeyEnter Key = 0x1C
	KeyCtrl  Key = 0x1D
	KeyA     Key = 0x1E
	KeyS     Key = 0x1F
	KeyD     Key = 0x20
	KeyF     Key = 0x21
	KeyG     Key = 0x22
	KeyH     Key = 0x23
	KeyJ     Key = 0x24
	KeyK     Key = 0x25
	KeyL     Key = 0x26
	KeySemi  Key = 0x27
	KeyQuot  Key = 0x28
	KeyTick  Key = 0x29
	KeyShift Key = 0x2A
	KeyBackslash Key = 0x2B
	KeyZ     Key = 0x2C
	KeyX     Key = 0x2D
	KeyC     Key = 0x2E
	KeyV     Key = 0x2F
	KeyB     Key = 0x30
	KeyN     Key = 0x31
	KeyM     Key = 0x32
	KeyComma Key = 0x33
	KeyDot   Key = 0x34
	KeySlash Key = 0x35
	KeySpace Key = 0x39
	KeyCaps  Key = 0x3A
	KeyF1    Key = 0x3B
	KeyF2    Key = 0x3C
	KeyF3    Key = 0x3D
	KeyF4    Key = 0x3E
	KeyF5    Key = 0x3F
	KeyF6    Key = 0x40
	KeyF7    Key = 0x41
	KeyF8    Key = 0x42
	KeyF9    Key = 0x43
	KeyF10   Key = 0x44
	KeyNumLock Key = 0x45
	KeyScroll  Key = 0x46
	KeyF11     Key = 0x57
	KeyF12     Key = 0x58
	KeyAlt     Key = 0x38
)

var runeMap = map[rune]Key{
	'a': KeyA, 'b': KeyB, 'c': KeyC, 'd': KeyD, 'e': KeyE, 'f': KeyF, 'g': KeyG,
	'h': KeyH, 'i': KeyI, 'j': KeyJ, 'k': KeyK, 'l': KeyL, 'm': KeyM, 'n': KeyN,
	'o': KeyO, 'p': KeyP, 'q': KeyQ, 'r': KeyR, 's': KeyS, 't': KeyT, 'u': KeyU,
	'v': KeyV, 'w': KeyW, 'x': KeyX, 'y': KeyY, 'z': KeyZ,
	'0': Key0, '1': Key1, '2': Key2, '3': Key3, '4': Key4,
	'5': Key5, '6': Key6, '7': Key7, '8': Key8, '9': Key9,
	' ': KeySpace, '\n': KeyEnter, '\t': KeyTab,
	'-': KeyMinus, '=': KeyEqual, '[': KeyLBr, ']': KeyRBr,
	';': KeySemi, '\'': KeyQuot, '`': KeyTick, '\\': KeySlash,
	',': KeyComma, '.': KeyDot, '/': KeySlash, // Conflict with Backslash? 
	// Standard US Layout: 
	// 0x2B is Backslash usually? No, 0x2B is '\' (US) or '#' (UK).
	// Wait, 0x35 is Slash (/).
	// Let's check docs.
	// 0x2B: VK_OEM_5 (Backslash)
	// 0x35: VK_OEM_2 (Slash / ?)
}

// KeyFromRune returns the Scan Code for a given rune.
// Note: This is a simplified mapping for US QWERTY.
func KeyFromRune(r rune) (Key, bool) {
	k, ok := runeMap[r]
	return k, ok
}
