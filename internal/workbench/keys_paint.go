package workbench

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

func (state *State) paintKeysWorkspace(canvas *ui.Canvas, bounds ui.Rect) {
	pal, messages := state.pal, state.messages
	importWidth := toolbarButtonWidth(canvas, messages.KeyImport, true)
	removeWidth := toolbarButtonWidth(canvas, messages.KeyRemove, false)
	statusColor, statusValue := pal.muted, ""
	if selected, ok := state.identityByID(state.selectedIdentityID); ok {
		statusValue = selected.Path
		if selected.Error != "" {
			statusColor, statusValue = pal.danger, selected.Error
		}
	}

	var content, importButton, removeButton flex.Box
	f := state.keysLayout.Begin(canvas, state.Theme(), bounds)
	f.Column().Fill().PadSides(12, 18, 0, 18).Children(func() {
		f.Row().Children(func() {
			f.Column().Grow(1).PadSides(3, 0, 0, 0).Children(func() {
				f.Text(messages.KeyTitle).FontSize(18).Weight(ui.WeightSemibold).Color(pal.text).LineHeight(28).Width(bounds.Width * 0.45)
				f.Paragraph(messages.KeyDescription, 2).FontSize(metrics.FontSizeS).Color(pal.muted).LineHeight(22).Width(bounds.Width * 0.62)
			})
			removeButton = f.Slot().Size(removeWidth, toolbarHeight).MarginSides(0, 10, 0, 0).Shrink(0)
			importButton = f.Slot().Size(importWidth, toolbarHeight).Shrink(0)
		})
		content = f.Slot().Grow(1).MarginSides(17, 0, 9, 0)
		f.Divider().Background(pal.line)
		f.Row().Height(50).PadSides(8, 0, 0, 0).Gap(4).Items(flex.Start).Children(func() {
			f.Text(state.identityCountText).FontSize(metrics.FontSizeS).Weight(ui.WeightMedium).Color(pal.faint).LineHeight(22).Width(120).Shrink(0)
			if statusValue != "" {
				f.Text(statusValue).FontSize(metrics.FontSizeS).Color(statusColor).LineHeight(22).Grow(1)
			}
		})
	})
	f.End()

	state.paintToolbarButton(canvas, hitKeysImport, importButton.Rect().X, importButton.Rect().Y, importWidth, messages.KeyImport, iconPlus, true, true)
	state.paintToolbarButton(canvas, hitKeysRemove, removeButton.Rect().X, removeButton.Rect().Y, removeWidth, messages.KeyRemove, 0, false, state.selectedIdentityID != 0)

	if len(state.identities) == 0 {
		state.paintEmptyKeys(canvas, content.Rect())
		return
	}
	columns := [...]components.TableColumn{{Width: 210}, {Width: 170}, {Width: 145}, {Width: 0}}
	headings := [...]string{messages.KeyColumnName, messages.KeyColumnType, messages.KeyColumnProtection, messages.KeyColumnFingerprint}
	components.FitTableHeadings(canvas, state.Theme(), columns[:], headings[:])
	table := components.BeginTable(canvas, state.Theme(), content.Rect(), columns[:])
	table.Header(headings[:])
	for _, identity := range state.identities {
		if table.RowAt(hitKeyRowList, int(identity.ID), identity.cells[:], identity.ID == state.selectedIdentityID) && state.modal == modalNone {
			state.selectIdentity(identity.ID)
		}
	}
}

// paintEmptyKeys centres the invitation to import a first key, a little above
// the middle of the area.
func (state *State) paintEmptyKeys(canvas *ui.Canvas, bounds ui.Rect) {
	pal, messages := state.pal, state.messages
	width := toolbarButtonWidth(canvas, messages.KeyImport, true)
	var tile, icon, button flex.Box
	f := state.keysEmptyLayout.Begin(canvas, state.Theme(), bounds)
	f.Column().Fill().Items(flex.Center).Children(func() {
		f.Spacer().HeightPercent(40).Grow(0).MarginSides(0, 0, -48, 0).Shrink(0)
		tile = f.Column().Size(62, 62).Center().Shrink(0).Children(func() {
			icon = f.Slot().Size(24, 24)
		})
		f.Text(messages.KeyEmpty).FontSize(17).Weight(ui.WeightSemibold).Color(pal.text).LineHeight(28).TextCenter().MarginSides(18, 0, 0, 0).Width(560)
		f.Text(messages.KeyEmptySub).FontSize(metrics.FontSizeM).Color(pal.muted).LineHeight(23).TextCenter().MarginSides(5, 0, 0, 0).Width(600)
		button = f.Slot().Size(width, toolbarHeight).MarginSides(18, 0, 0, 0)
	})
	f.End()
	canvas.Rounded(tile.Rect(), 15, pal.surface)
	paintIcon(canvas, iconKey, icon.Rect(), pal.muted)
	state.paintToolbarButton(canvas, hitKeysImport, button.Rect().X, button.Rect().Y, width, messages.KeyImport, iconPlus, true, true)
}
