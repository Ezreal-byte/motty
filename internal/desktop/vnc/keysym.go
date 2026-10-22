package vnc

import "motty/internal/desktop"

// keysymFor is the X11 keysym RFB names a key with: the Latin-1 characters are
// their own code points, other characters are 0x01000000 plus the code point,
// and the keys that are not characters have their own numbers.
func keysymFor(event desktop.KeyEvent) uint32 {
	if event.Rune != 0 {
		r := event.Rune
		if event.Typed != 0 {
			r = event.Typed
		}
		switch {
		case r < 0x20 || r == 0x7f || r > 0x10ffff:
			return 0
		case r < 0x100:
			return uint32(r)
		}
		return 0x01000000 + uint32(r)
	}
	if event.Key >= desktop.KeyF1 && event.Key <= desktop.KeyF12 {
		return 0xffbe + uint32(event.Key-desktop.KeyF1)
	}
	switch event.Key {
	case desktop.KeyEnter:
		return 0xff0d
	case desktop.KeyTab:
		return 0xff09
	case desktop.KeyBackspace:
		return 0xff08
	case desktop.KeyEscape:
		return 0xff1b
	case desktop.KeyDelete:
		return 0xffff
	case desktop.KeyInsert:
		return 0xff63
	case desktop.KeyHome:
		return 0xff50
	case desktop.KeyEnd:
		return 0xff57
	case desktop.KeyPageUp:
		return 0xff55
	case desktop.KeyPageDown:
		return 0xff56
	case desktop.KeyLeft:
		return 0xff51
	case desktop.KeyUp:
		return 0xff52
	case desktop.KeyRight:
		return 0xff53
	case desktop.KeyDown:
		return 0xff54
	case desktop.KeyShift:
		return 0xffe1
	case desktop.KeyControl:
		return 0xffe3
	case desktop.KeyAlt:
		return 0xffe9
	case desktop.KeyMeta:
		return 0xffeb // Super_L
	case desktop.KeyCapsLock:
		return 0xffe5
	case desktop.KeyMenu:
		return 0xff67
	}
	return 0
}
