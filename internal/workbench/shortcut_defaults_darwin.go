//go:build darwin

package workbench

import "github.com/vivaui/viva/ui"

func defaultShortcutModifier() ui.Modifiers { return ui.ModifierMeta }
