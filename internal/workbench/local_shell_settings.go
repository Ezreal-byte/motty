package workbench

import (
	"path/filepath"
	"runtime"
	"strings"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
	"motty/internal/localshell"
)

var (
	hitSettingsLocalShell        = tree.ID(settingsHits, "settings-local-shell")
	hitSettingsLocalShellDismiss = tree.ID(settingsHits, "settings-local-shell-dismiss")
	hitSettingsLocalShellItems   = tree.Sub(settingsHits, tree.Name("settings-local-shell-item"))
)

type localShellSelect struct {
	choices  []localshell.Choice
	loaded   bool
	open     bool
	bounds   ui.Rect
	discover func() []localshell.Choice // nil uses the Windows discovery
}

func localShellSettingVisible() bool { return runtime.GOOS == "windows" }

func (state *State) refreshLocalShells() {
	menu := &state.localShellMenu
	discover := menu.discover
	if discover == nil {
		discover = localshell.InstalledWindowsShells
	}
	menu.choices, menu.loaded = discover(), true
}

func (state *State) localShellSelected() int {
	for index, choice := range state.localShellMenu.choices {
		if state.localShell == "" && choice.Default || strings.EqualFold(choice.Path, state.localShell) {
			return index
		}
	}
	return -1
}

func (state *State) paintLocalShellSetting(page *settingsPage) {
	if !localShellSettingVisible() {
		return
	}
	if !state.localShellMenu.loaded {
		state.refreshLocalShells()
	}
	page.add(settingsControl{kind: controlLocalShellSelect, slot: page.rowFrame(state.messages.SettingsLocalShell, state.messages.SettingsLocalShellDescription)})
}

// Paint the select in the menu pass, outside the settings page's clip.
func (state *State) paintLocalShellSelect(canvas *ui.Canvas) {
	menu, m := &state.localShellMenu, state.messages
	bounds := menu.bounds
	if !localShellSettingVisible() || state.modal != modalNone || bounds.Width <= 0 || bounds.Y < windowBarHeight || bounds.Y+bounds.Height > state.viewport.Height {
		menu.open = false
		return
	}
	selected := state.localShellSelected()
	value, hint := m.SettingsLocalShellEmpty, ""
	if selected >= 0 {
		value, hint = menu.choices[selected].Name, menu.choices[selected].Path
	} else if state.localShell != "" {
		value, hint = filepath.Base(state.localShell), state.localShell
	}
	below := max(state.viewport.Height-8-bounds.Y-bounds.Height-4, 0)
	above := max(bounds.Y-windowBarHeight-8, 0)
	height := min(float32(8+len(menu.choices)*32), max(above, below))
	content := ui.Rect{X: bounds.X, Y: bounds.Y + bounds.Height + 4, Width: bounds.Width, Height: height}
	if below < height {
		content.Y = bounds.Y - height - 4
	}
	opening := menu.open && state.popups.owner != hitSettingsLocalShellDismiss
	scroll := state.selectScroll(menu.open, hitSettingsLocalShellDismiss)
	if opening && selected >= 0 {
		scroll.Offset = max(0, min(float32(selected)*32, float32(len(menu.choices)*32)+8-height))
		canvas.SetInitialFocus(hitSettingsLocalShellItems.At(selected))
	}
	control := components.BeginSelect(canvas, state.Theme(), bounds, components.SelectOptions{
		TriggerID: hitSettingsLocalShell, Label: m.SettingsLocalShell, Value: value, Hint: hint,
		Disabled: len(menu.choices) == 0, Open: menu.open, Content: content, Layer: popupLayer,
		Scope: hitSettingsLocalShellDismiss, DismissID: hitSettingsLocalShellDismiss, Viewport: state.windowRect(), Scroll: scroll,
	})
	defer control.End()
	if menu.open {
		state.menuPlaced(control.ContentBounds())
		for index, choice := range menu.choices {
			control.AddItem(components.SelectItem{ID: hitSettingsLocalShellItems.At(index), Label: choice.Name, Selected: index == selected})
		}
	}
}

func (state *State) clickLocalShell(id ui.ID) bool {
	if !localShellSettingVisible() || state.settingsCategory != settingsTerminal || state.settingsSearching() {
		return false
	}
	menu := &state.localShellMenu
	switch id {
	case hitSettingsLocalShell:
		if !menu.open {
			state.refreshLocalShells()
		}
		menu.open = !menu.open && len(menu.choices) > 0
		return true
	case hitSettingsLocalShellDismiss:
		menu.open = false
		return true
	}
	index, ok := id.In(hitSettingsLocalShellItems)
	if !ok || index < 0 || index >= len(menu.choices) {
		return false
	}
	program := menu.choices[index].Path
	menu.open = false
	if state.localShell != program {
		state.localShell = program
		state.save()
	}
	return true
}
