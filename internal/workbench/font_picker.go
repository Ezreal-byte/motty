package workbench

import (
	"fmt"
	"strings"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// The terminal's font. The two built-in faces are always there; any installed
// fixed-pitch family can be chosen from a list that is read from the system when
// the picker is first opened (reading it takes a moment, so never on a paint).

// fontPickerHits is the place of the controls this file paints.
var fontPickerHits = tree.Feature("font-picker")

var (
	hitFontPickerSearch   = tree.ID(fontPickerHits, "font-picker-search")
	hitFontPickerDismiss  = tree.ID(fontPickerHits, "font-picker-dismiss")
	hitFontPickerClose    = tree.ID(fontPickerHits, "font-picker-close")
	hitSettingsFontPick   = tree.ID(fontPickerHits, "settings-font-pick")
	hitFontPickerItemList = tree.Sub(fontPickerHits, tree.Name("font-picker-item"))
)

const fontPickerVisibleRows = 8

// fontEntry is one choice: a built-in face (family empty) or an installed family.
type fontEntry struct {
	label  string
	family string
	face   ui.MonoFace // for built-in entries
}

type fontsUI struct {
	// family is the installed family in use, "" for a built-in face.
	family string
	// installed is what the system listed; nil until it has been read.
	installed []string
	loading   bool

	search   text.Editor
	raw      string // the search text the filter was built for
	entries  []fontEntry
	filtered []int32 // indices into entries
	selected int     // index into filtered
	top      int     // first filtered row shown
	built    bool

	// text is the description the settings row shows; textFor is what it was made for.
	text, textFor string
}

// fontName is the name of the font in use, in the current language.
func (state *State) fontName() string {
	switch {
	case state.fonts.family != "":
		return state.fonts.family
	case state.terminalFontFace == ui.MonoClassic:
		return state.messages.SettingsTerminalFontClassic
	}
	return state.messages.SettingsTerminalFontSystem
}

// fontDescription is the settings row's explanation: what the font is now. It is
// made again only when that changes.
func (state *State) fontDescription() string {
	name := state.fontName()
	if state.fonts.textFor != name || state.fonts.text == "" {
		state.fonts.textFor = name
		state.fonts.text = fmt.Sprintf(state.messages.Tools.FontCurrentFmt, name)
	}
	return state.fonts.text
}

// listFonts reads the installed families off the window thread and delivers the
// list to fn on it.
func (state *State) listFonts(fn func(*State, []string)) {
	list := state.options.MonoFamilies
	if !state.aiPost(func(*State) {}) { // the window is closing
		return
	}
	go func() {
		families := list()
		state.aiPost(func(s *State) { fn(s, families) })
	}()
}

// validateTerminalFont checks, once the list can be read, that the saved family
// is still installed, and falls back to the system face when it is not.
func (state *State) validateTerminalFont() {
	if state.fonts.family == "" {
		return
	}
	family := state.fonts.family
	state.listFonts(func(s *State, families []string) {
		s.fonts.installed = families
		s.fonts.built = false
		if s.fonts.family != family {
			return // the user chose another while the list was being read
		}
		for _, name := range families {
			if name == family {
				return
			}
		}
		s.fonts.family = ""
		s.setTerminalFontFace(ui.MonoSystem)
		s.status = fmt.Sprintf(s.messages.Tools.FontMissingFmt, family)
		s.save()
	})
}

func (state *State) openFontPicker() {
	state.modal = modalFontPicker
	state.fonts.search = text.Editor{}
	state.fonts.built = false
	if state.fonts.installed == nil && !state.fonts.loading {
		state.fonts.loading = true
		state.listFonts(func(s *State, families []string) {
			s.fonts.loading = false
			if families == nil {
				families = []string{}
			}
			s.fonts.installed = families
			s.fonts.built = false
		})
	}
}

// refreshFontPicker builds the list the picker shows for its search text.
func (state *State) refreshFontPicker() {
	fonts := &state.fonts
	raw := fonts.search.String()
	if fonts.built && raw == fonts.raw {
		return
	}
	first := !fonts.built
	fonts.built, fonts.raw = true, raw
	fonts.entries = fonts.entries[:0]
	fonts.entries = append(fonts.entries,
		fontEntry{label: state.messages.SettingsTerminalFontSystem, face: ui.MonoSystem},
		fontEntry{label: state.messages.SettingsTerminalFontClassic, face: ui.MonoClassic})
	for _, name := range fonts.installed {
		fonts.entries = append(fonts.entries, fontEntry{label: name, family: name})
	}
	query := strings.ToLower(strings.TrimSpace(raw))
	fonts.filtered = fonts.filtered[:0]
	current := -1
	for index, entry := range fonts.entries {
		if query != "" && !strings.Contains(strings.ToLower(entry.label), query) {
			continue
		}
		if entry.family == fonts.family && (entry.family != "" || entry.face == state.terminalFontFace) {
			current = len(fonts.filtered)
		}
		fonts.filtered = append(fonts.filtered, int32(index))
	}
	fonts.selected, fonts.top = 0, 0
	if first && current >= 0 {
		fonts.selected = current
		fonts.top = max(min(current-fontPickerVisibleRows/2, len(fonts.filtered)-fontPickerVisibleRows), 0)
	}
}

// moveFontSelection moves the highlighted row and keeps it in view.
func (state *State) moveFontSelection(delta int) {
	fonts := &state.fonts
	state.refreshFontPicker()
	if len(fonts.filtered) == 0 {
		return
	}
	fonts.selected = min(max(fonts.selected+delta, 0), len(fonts.filtered)-1)
	if fonts.selected < fonts.top {
		fonts.top = fonts.selected
	}
	if fonts.selected >= fonts.top+fontPickerVisibleRows {
		fonts.top = fonts.selected - fontPickerVisibleRows + 1
	}
}

func (state *State) scrollFontPicker(event ui.ScrollEvent) ui.EventResult {
	fonts := &state.fonts
	state.refreshFontPicker()
	before := fonts.top
	step := 1
	if event.DeltaY < 0 {
		step = -1
	}
	fonts.top = min(max(fonts.top+step, 0), max(len(fonts.filtered)-fontPickerVisibleRows, 0))
	return ui.EventResult{Handled: true, Repaint: fonts.top != before}
}

// chooseFont applies the highlighted (or clicked) row and closes the picker.
func (state *State) chooseFont(row int) {
	fonts := &state.fonts
	state.refreshFontPicker()
	if row < 0 || row >= len(fonts.filtered) {
		return
	}
	entry := fonts.entries[fonts.filtered[row]]
	state.closeModal()
	if state.useTerminalFont(entry) {
		state.save()
	}
}

// useTerminalFont switches the terminal to a font. It reports whether anything changed.
func (state *State) useTerminalFont(entry fontEntry) bool {
	if entry.family == "" {
		changed := state.fonts.family != ""
		state.fonts.family = ""
		return state.setTerminalFontFace(entry.face) || changed
	}
	face := ui.MonoFamily(entry.family)
	if face == ui.MonoSystem {
		return false
	}
	changed := state.fonts.family != entry.family
	state.fonts.family = entry.family
	return state.setTerminalFontFace(face) || changed
}

func (state *State) clickFontPicker(id ui.ID) {
	switch {
	case id == hitFontPickerDismiss || id == hitFontPickerClose:
		state.closeModal()
	case id.Scope() == hitFontPickerItemList:
		state.refreshFontPicker()
		state.chooseFont(state.fonts.top + item(id))
	}
}

// fontPickerKey handles the keys of the picker. It reports whether it took the key.
func (state *State) fontPickerKey(event ui.KeyEvent) (ui.EventResult, bool) {
	if state.modal != modalFontPicker {
		return ui.EventResult{}, false
	}
	handled := ui.EventResult{Handled: true, Repaint: true}
	switch event.Key {
	case ui.KeyEscape:
		state.closeModal()
		return handled, true
	case ui.KeyUp:
		state.moveFontSelection(-1)
		return handled, true
	case ui.KeyDown:
		state.moveFontSelection(1)
		return handled, true
	case ui.KeyPageUp:
		state.moveFontSelection(-fontPickerVisibleRows)
		return handled, true
	case ui.KeyPageDown:
		state.moveFontSelection(fontPickerVisibleRows)
		return handled, true
	case ui.KeyEnter:
		if !event.Repeat {
			state.chooseFont(state.fonts.selected)
		}
		return handled, true
	}
	return ui.EventResult{}, false
}

func (state *State) paintFontPicker(canvas *ui.Canvas, viewport ui.Rect) {
	state.refreshFontPicker()
	m, pal, fonts := state.messages.Tools, state.pal, &state.fonts
	dialog := components.BeginDialog(canvas, state.Theme(), viewport, components.DialogOptions{
		Layer: dialogLayer, Scope: dialogScope,
		DismissID: hitFontPickerDismiss, CloseID: hitFontPickerClose, InitialFocusID: hitFontPickerSearch,
		ContentSize: ui.Size{Width: 560, Height: 502},
	})
	layout := dialog.Begin(m.FontPickerTitle, "")
	rows := max(min(len(fonts.filtered)-fonts.top, fontPickerVisibleRows), 0)
	var search flex.Box
	// The list keeps room for all its rows, so the sample under it stays where it is;
	// what the list lacks is said under the rows it has.
	f := state.fontPickerLayout.Begin(canvas, state.Theme(), layout.Body)
	f.Column().Fill().Children(func() {
		search = f.Slot().Height(36)
		f.Column().Height(fontPickerVisibleRows*32).Shrink(0).MarginSides(10, 0, 0, 0).Children(func() {
			for visible := 0; visible < rows; visible++ {
				row := fonts.top + visible
				entry := fonts.entries[fonts.filtered[row]]
				selected := row == fonts.selected
				inUse := entry.family == fonts.family && (entry.family != "" || entry.face == state.terminalFontFace)
				color := pal.text
				line := f.Row().Height(30).Shrink(0).MarginSides(0, 0, 2, 0).PadSides(0, 12, 0, 10).Items(flex.Center).Radius(radius.item).
					HitWith(hitFontPickerItemList.At(visible), entry.label, ui.HitOptions{Role: ui.RoleButton, Selected: selected})
				if selected {
					line.Background(pal.accent)
					color = pal.onAccent
				} else {
					line.HoverBackground(pal.surfaceHot)
				}
				line.Children(func() {
					f.Text(entry.label).FontSize(metrics.FontSizeM).Weight(ui.WeightMedium).Color(color).LineHeight(20).Grow(1).MarginSides(0, 12, 0, 0)
					if inUse {
						f.Icon(lucideIcons[iconCheck]).Size(16, 16).Color(color)
					} else {
						f.Spacer().Size(16, 16).Grow(0)
					}
				})
			}
			switch {
			case fonts.loading:
				f.Text(m.FontPickerLoading).FontSize(metrics.FontSizeM).Color(pal.muted).LineHeight(24).TextCenter().MarginSides(10, 0, 0, 0)
			case len(fonts.filtered) == 0:
				f.Text(m.FontPickerEmpty).FontSize(metrics.FontSizeM).Color(pal.muted).LineHeight(24).TextCenter().MarginSides(10, 0, 0, 0)
			}
		})
		if len(fonts.filtered) > 0 {
			// The highlighted font, set in itself.
			entry := fonts.entries[fonts.filtered[fonts.selected]]
			face := entry.face
			if entry.family != "" {
				face = ui.MonoFamily(entry.family)
			}
			f.Column().Height(56).Shrink(0).PadSides(7, 11, 0, 11).Background(pal.terminal).Border(1, pal.line).Radius(radius.control).MarginSides(8, 0, 0, 0).Children(func() {
				f.Text(m.FontPickerSample).FontSize(metrics.FontSizeM).Color(pal.text).MonoFace(face).LineHeight(20).Height(20)
				f.Text(m.FontPickerSampleSymbols).FontSize(metrics.FontSizeM).Color(pal.muted).MonoFace(face).LineHeight(20).Height(20)
			})
		}
	})
	f.End()

	value := fonts.search.String()
	focused := canvas.Focused() == hitFontPickerSearch
	components.PaintInput(canvas, state.Theme(), search.Rect(), components.InputSnapshot{
		ID: hitFontPickerSearch, Label: m.FontPickerTitle, Value: value, Placeholder: m.FontPickerPlaceholder,
		Focused: focused, PlatformEditing: focused && value != "",
	})
	dialog.End()
}
