package workbench

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

const commandPaletteVisibleRows = 10

func (state *State) paintCommandPalette(canvas *ui.Canvas, viewport ui.Rect) {
	state.refreshCommandPalette()
	dialog := components.BeginDialog(canvas, state.Theme(), viewport, components.DialogOptions{
		Layer: dialogLayer, Scope: dialogScope,
		DismissID: hitCommandPaletteDismiss, CloseID: hitCommandPaletteClose, InitialFocusID: hitCommandPaletteSearch,
		ContentSize: ui.Size{Width: 620, Height: 446},
	})
	title, placeholder, empty := state.messages.CommandPaletteTitle, state.messages.CommandPalettePlaceholder, state.messages.CommandPaletteEmpty
	if state.commandPalette.connect {
		title, placeholder, empty = state.messages.LabelNewTab, state.messages.NewTabPickerPlaceholder, state.messages.NewTabPickerEmpty
	}
	layout := dialog.Begin(title, "")
	value := state.commandPalette.search.String()
	focused := canvas.Focused() == hitCommandPaletteSearch
	palette := &state.commandPalette
	palette.follow()
	count := min(int(palette.visibleCount)-int(palette.top), commandPaletteVisibleRows)
	// One row is the current one. The pointer on a row makes it that one, so the
	// row the keyboard last chose does not stay lit beside it.
	current := -1
	for visible := 0; visible < count; visible++ {
		if canvas.Hovered() == hitCommandPaletteItems.At(visible) {
			current = visible
		}
	}

	var search flex.Box
	f := state.paletteLayout.Begin(canvas, state.Theme(), layout.Body)
	f.Column().Fill().Children(func() {
		search = f.Slot().Height(36).FillWidth().Shrink(0)
		if count == 0 {
			f.Text(empty).FontSize(metrics.FontSizeM).Color(state.pal.muted).LineHeight(24).TextCenter().MarginSides(36, 0, 0, 0)
			return
		}
		f.Column().Gap(4).MarginSides(12, 0, 0, 0).Children(func() {
			for visible := 0; visible < count; visible++ {
				entry := palette.entries[palette.visible[int(palette.top)+visible]]
				id := hitCommandPaletteItems.At(visible)
				selected := current == visible || current < 0 && (palette.selected == palette.top+uint8(visible) || canvas.Focused() == id)
				color, detailColor := state.pal.muted, state.pal.faint
				if selected {
					color, detailColor = state.pal.onAccent, state.pal.onAccent
				}
				row := f.Row().Height(30).PadX(10).Gap(12).Items(flex.Center).Shrink(0)
				if selected {
					row.Background(state.pal.accent).Radius(radius.item)
				}
				row.HitWith(id, entry.label, ui.HitOptions{Role: ui.RoleButton}).Children(func() {
					f.Text(entry.label).FontSize(metrics.FontSizeM).Weight(ui.WeightMedium).Color(color).Ellipsis().Grow(1)
					if entry.detail != "" {
						f.Text(entry.detail).FontSize(metrics.FontSizeS).Color(detailColor).Ellipsis().MaxWidth(layout.Body.Width * 0.45).Shrink(0)
					}
				})
			}
		})
	})
	f.End()
	if total := int(palette.visibleCount); total > commandPaletteVisibleRows {
		// A thumb on the right edge says there are more rows above or below.
		track := ui.Rect{X: layout.Body.X + layout.Body.Width - 4, Y: search.Rect().Y + 48, Width: 3, Height: commandPaletteVisibleRows*34 - 4}
		thumb := track.Height * commandPaletteVisibleRows / float32(total)
		at := track.Y + (track.Height-thumb)*float32(palette.top)/float32(total-commandPaletteVisibleRows)
		canvas.Rounded(ui.Rect{X: track.X, Y: at, Width: track.Width, Height: thumb}, 1.5, withAlpha(state.pal.muted, 0.6))
	}
	components.PaintInput(canvas, state.Theme(), search.Rect(), components.InputSnapshot{
		ID: hitCommandPaletteSearch, Label: title, Value: value,
		Placeholder: placeholder, Focused: focused, PlatformEditing: focused && value != "",
	})
	dialog.End()
}
