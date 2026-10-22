package rdp

import "motty/internal/desktop"

// Keys are sent as scancodes of the PC/AT set 1, which every RDP server maps
// through the keyboard layout it was told about (US English). An extended key
// (the grey ones: arrows, Home, the Windows key) is 0xE000 plus its code, the
// form rdpcore.Client.SendKeyboard takes.
const extended = 0xE000

// scancode is a key's code and whether Shift has to be down for the character
// asked for on a US keyboard.
type scancode struct {
	code  uint16
	shift bool
}

// namedKeys are the keys that are not characters.
var namedKeys = [...]uint16{
	desktop.KeyEnter:     0x1C,
	desktop.KeyTab:       0x0F,
	desktop.KeyBackspace: 0x0E,
	desktop.KeyEscape:    0x01,
	desktop.KeyDelete:    extended | 0x53,
	desktop.KeyInsert:    extended | 0x52,
	desktop.KeyHome:      extended | 0x47,
	desktop.KeyEnd:       extended | 0x4F,
	desktop.KeyPageUp:    extended | 0x49,
	desktop.KeyPageDown:  extended | 0x51,
	desktop.KeyLeft:      extended | 0x4B,
	desktop.KeyRight:     extended | 0x4D,
	desktop.KeyUp:        extended | 0x48,
	desktop.KeyDown:      extended | 0x50,
	desktop.KeyShift:     0x2A,
	desktop.KeyControl:   0x1D,
	desktop.KeyAlt:       0x38,
	desktop.KeyMeta:      extended | 0x5B,
	desktop.KeyCapsLock:  0x3A,
	desktop.KeyMenu:      extended | 0x5D,
}

// keyCode is the scancode of a named key, 0 if it has none.
func keyCode(key desktop.Key) uint16 {
	if key >= desktop.KeyF1 && key <= desktop.KeyF12 {
		index := uint16(key - desktop.KeyF1)
		if index < 10 {
			return 0x3B + index
		}
		return 0x57 + index - 10 // F11, F12
	}
	if int(key) < len(namedKeys) {
		return namedKeys[key]
	}
	return 0
}

// The rows of a US keyboard, unshifted and with Shift held, from the scancode
// each starts at.
var usRows = [...]struct {
	first          uint16
	plain, shifted string
}{
	{0x02, "1234567890-=", "!@#$%^&*()_+"},
	{0x10, "qwertyuiop[]", "QWERTYUIOP{}"},
	{0x1E, "asdfghjkl;'`", "ASDFGHJKL:\"~"},
	{0x2B, "\\", "|"},
	{0x2C, "zxcvbnm,./", "ZXCVBNM<>?"},
	{0x39, " ", " "},
}

var runeCodes = func() map[rune]scancode {
	table := make(map[rune]scancode, 100)
	for _, row := range usRows {
		for index, r := range row.plain {
			table[r] = scancode{code: row.first + uint16(index)}
		}
		for index, r := range row.shifted {
			if _, taken := table[r]; !taken {
				table[r] = scancode{code: row.first + uint16(index), shift: true}
			}
		}
	}
	return table
}()

// runeCode is the key that types r on a US keyboard.
func runeCode(r rune) (scancode, bool) {
	code, ok := runeCodes[r]
	return code, ok
}
