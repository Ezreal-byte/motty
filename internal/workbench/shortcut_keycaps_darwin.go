//go:build darwin

package workbench

import "github.com/vivaui/viva/ui"

// modifierKeycap is the symbol a Mac keyboard prints on a modifier key.
func modifierKeycap(modifier ui.Modifiers) string {
	switch modifier {
	case ui.ModifierControl:
		return "⌃"
	case ui.ModifierAlt:
		return "⌥"
	case ui.ModifierShift:
		return "⇧"
	default:
		return "⌘"
	}
}
