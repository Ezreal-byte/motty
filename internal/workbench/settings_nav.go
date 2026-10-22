package workbench

import (
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// The settings page is a list of categories on the left and one category's
// groups on the right, so a long page is never one long scroll.

type settingsCategory uint8

const (
	settingsGeneral settingsCategory = iota
	settingsTerminal
	settingsConnection
	settingsFiles
	settingsSecurity
	settingsBackup
	settingsAssistant
	settingsMCP
	settingsShortcuts
	settingsCategoryCount
)

// settingsNavHits is the place of the controls this file paints.
var settingsNavHits = tree.Feature("settings-nav")

// hitSettingsCategoryList starts the ids of the category entries.
var hitSettingsCategoryList = tree.Sub(settingsNavHits, tree.Name("settings-category"))

const settingsNavWidthMax = float32(208)

func (category settingsCategory) icon() uint8 {
	switch category {
	case settingsTerminal:
		return iconTerminal
	case settingsConnection:
		return iconLink
	case settingsFiles:
		return iconFolderOpen
	case settingsSecurity:
		return iconLock
	case settingsBackup:
		return iconArchive
	case settingsAssistant:
		return iconBot
	case settingsMCP:
		return iconPlug
	case settingsShortcuts:
		return iconKeyboard
	}
	return iconSettings
}

// settingsCategoryName is a category's name, in the list and as the page's title.
func (state *State) settingsCategoryName(category settingsCategory) string {
	m := state.messages.Tools
	switch category {
	case settingsTerminal:
		return m.CatTerminal
	case settingsConnection:
		return m.CatConnection
	case settingsFiles:
		return m.CatFiles
	case settingsSecurity:
		return m.CatSecurity
	case settingsBackup:
		return m.CatBackup
	case settingsAssistant:
		return state.messages.AISettings.Title
	case settingsMCP:
		return m.CatMCP
	case settingsShortcuts:
		return state.messages.SettingsShortcuts
	}
	return m.CatGeneral
}

func (state *State) selectSettingsCategory(category settingsCategory) bool {
	if category >= settingsCategoryCount || state.settingsCategory == category {
		return false
	}
	state.settingsCategory = category
	state.settingsScroll = 0
	return true
}

// nav declares the category list: the page's title, the search box and one entry
// per category, in a column of the given width.
func (page *settingsPage) nav(width float32, searching bool) {
	state, f := page.state, page.f
	pal, m := state.pal, state.messages
	f.Column().Width(width).FillHeight().Children(func() {
		f.Text(m.SettingsTitle).FontSize(19).Bold().Color(pal.text).LineHeight(28).MarginSides(26, 8, 10, 22).Shrink(0)
		page.add(settingsControl{kind: controlSearch, slot: f.Slot().Height(38).MarginSides(0, 12, 12, 12).Shrink(0)})
		f.Column().Gap(4).Shrink(0).Children(func() {
			for category := settingsCategory(0); category < settingsCategoryCount; category++ {
				page.navEntry(hitSettingsCategoryList.At(int(category)), category.icon(), state.settingsCategoryName(category), !searching && category == state.settingsCategory)
			}
		})
	})
}

// navEntry declares one entry of the list: it fills while it is the one shown or
// under the pointer, and its icon and name are brighter then. The entry is a
// pointer target recorded with the page's controls, so the keyboard visits it
// after the search box above it.
func (page *settingsPage) navEntry(id ui.ID, icon uint8, label string, active bool) {
	f, pal := page.f, page.state.pal
	hovered := f.Canvas().Hovered() == id
	foreground, background := pal.muted, ui.Color{}
	switch {
	case active:
		foreground, background = pal.text, pal.surfaceHot
	case hovered:
		foreground, background = pal.text, pal.surfaceSoft
	}
	entry := f.Row().Height(36).Shrink(0).MarginX(12).PadSides(0, 8, 0, 12).Gap(12).Items(flex.Center).Radius(radius.control).Background(background)
	page.hitOn(entry, id, label, ui.HitOptions{Role: ui.RoleMenuItem, Selected: active})
	entry.Children(func() {
		f.Icon(lucideIcons[icon]).Color(foreground)
		f.Text(label).FontSize(metrics.FontSizeM).Weight(ui.WeightMedium).Color(foreground).LineHeight(22).Grow(1)
	})
}
