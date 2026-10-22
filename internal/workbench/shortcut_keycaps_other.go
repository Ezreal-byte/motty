//go:build !darwin

package workbench

import "github.com/vivaui/viva/ui"

// modifierKeycap is the name printed on a modifier key.
func modifierKeycap(modifier ui.Modifiers) string {
	switch modifier {
	case ui.ModifierControl:
		return "Ctrl"
	case ui.ModifierAlt:
		return "Alt"
	case ui.ModifierShift:
		return "Shift"
	default:
		return "Win"
	}
}
