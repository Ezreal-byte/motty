package workbench

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

func (state *State) paintKnownHostsWorkspace(canvas *ui.Canvas, bounds ui.Rect) {
	state.syncKnownHostsVisible()
	view, messages, pal := &state.knownHosts, state.messages, state.pal
	value := view.search.String()
	deleteWidth := toolbarButtonWidth(canvas, messages.KnownHostsDelete, false)
	retrustWidth := toolbarButtonWidth(canvas, messages.KnownHostsRetrust, false)
	refreshWidth := toolbarButtonWidth(canvas, messages.KnownHostsRefresh, false)
	statusColor := pal.muted
	if view.errorText != "" {
		statusColor = pal.danger
	}
	paginated := len(view.visible) > knownHostsPageSize

	var search, content, pagination, refresh, retrust, remove flex.Box
	f := state.knownHostsLayout.Begin(canvas, state.Theme(), bounds)
	f.Column().Fill().PadSides(12, 18, 0, 18).Children(func() {
		f.Column().MarginSides(4, 0, 12, 0).Gap(3).Children(func() {
			f.Text(messages.KnownHostsTitle).FontSize(18).Weight(ui.WeightSemibold).Color(pal.text)
			f.Paragraph(messages.KnownHostsDescription, 2).FontSize(metrics.FontSizeS).Color(pal.muted)
		})
		f.Row().Height(40).Gap(8).Items(flex.Center).Children(func() {
			search = f.Slot().Grow(1).MaxWidth(470).Height(40)
			f.Spacer()
			refresh = f.Slot().Size(refreshWidth, toolbarHeight)
			retrust = f.Slot().Size(retrustWidth, toolbarHeight)
			remove = f.Slot().Size(deleteWidth, toolbarHeight)
		})
		content = f.Slot().Grow(1).MarginSides(14, 0, 0, 0)
		f.Divider().Background(pal.line)
		f.Row().Height(49).Gap(12).Items(flex.Center).Children(func() {
			f.Text(view.countText).FontSize(metrics.FontSizeS).Weight(ui.WeightMedium).Color(pal.faint).Width(168)
			f.Text(view.selectionText).FontSize(metrics.FontSizeS).Weight(ui.WeightMedium).Color(pal.muted).Width(138)
			f.Text(view.status).FontSize(metrics.FontSizeS).Color(statusColor).Grow(1)
			if paginated {
				pagination = f.Slot().Size(242, 30)
			}
		})
	})
	f.End()

	state.paintToolbarButton(canvas, hitKnownHostsRefresh, refresh.Rect().X, refresh.Rect().Y, refreshWidth, messages.KnownHostsRefresh, 0, false, true)
	state.paintToolbarButton(canvas, hitKnownHostsRetrust, retrust.Rect().X, retrust.Rect().Y, retrustWidth, messages.KnownHostsRetrust, 0, false, view.selected > 0)
	state.paintToolbarButton(canvas, hitKnownHostsDelete, remove.Rect().X, remove.Rect().Y, deleteWidth, messages.KnownHostsDelete, 0, false, view.selected > 0)
	components.PaintInputGroup(canvas, state.Theme(), search.Rect(), components.InputGroupSnapshot{
		ID: hitKnownHostsSearch, Label: messages.NavSearch, Value: value, Placeholder: messages.KnownHostsSearchPlaceholder,
		NativeEditing: canvas.Focused() == hitKnownHostsSearch && value != "",
		InlineStart:   components.InputGroupSlot{Kind: components.InputGroupSlotIcon, Icon: lucideIcons[iconSearch]},
	})

	switch {
	case view.errorText != "":
		state.paintKnownHostsEmpty(canvas, content.Rect(), messages.KnownHostsError, view.errorText, pal.danger)
	case len(view.rows) == 0:
		state.paintKnownHostsEmpty(canvas, content.Rect(), messages.KnownHostsEmpty, messages.KnownHostsEmptySub, pal.muted)
	case len(view.visible) == 0:
		state.paintKnownHostsEmpty(canvas, content.Rect(), messages.KnownHostsNoMatches, messages.KnownHostsNoMatchesSub, pal.muted)
	default:
		state.paintKnownHostsTable(canvas, content.Rect())
	}
	if paginated {
		bar := components.BeginPagination(canvas, state.Theme(), pagination.Rect())
		bar.AddPrevious(hitKnownHostsPrevious, messages.KnownHostsPrevious, view.page == 0)
		bar.AddPage(ui.ID{}, view.pageText, true)
		pages := (len(view.visible) + knownHostsPageSize - 1) / knownHostsPageSize
		bar.AddNext(hitKnownHostsNext, messages.KnownHostsNext, view.page+1 >= pages)
	}
}

func (state *State) paintKnownHostsTable(canvas *ui.Canvas, bounds ui.Rect) {
	view, messages := &state.knownHosts, state.messages
	columns := [...]components.TableColumn{
		{Width: 42}, {Width: 46, Alignment: components.TableAlignRight}, {Width: 230},
		{Width: 102}, {Width: 142}, {Width: 300}, {Width: 0},
	}
	start, end := state.knownHostsPageRange()
	theme := state.Theme()
	rowHeight, headerHeight := components.TableRowHeight(theme), components.TableHeaderHeight(theme)
	checkSize := 16 * theme.Metrics.ControlHeight / 32

	// The checkbox of each row sits in the table's first column, below its header.
	var checks [knownHostsPageSize]flex.Box
	f := state.knownHostsTableLayout.Begin(canvas, state.Theme(), bounds)
	f.Column().Fill().PadSides(headerHeight, 0, 0, 0).Children(func() {
		for offset := range view.visible[start:end] {
			f.Row().Height(rowHeight).Shrink(0).Items(flex.Center).PadSides(0, 0, 0, 13).Children(func() {
				checks[offset] = f.Slot().Size(checkSize, checkSize)
			})
		}
	})
	f.End()

	headings := [...]string{
		"", messages.KnownHostsColumnLine, messages.KnownHostsColumnHosts, messages.KnownHostsColumnFlags,
		messages.KnownHostsColumnType, messages.KnownHostsColumnFingerprint, messages.KnownHostsColumnComment,
	}
	components.FitTableHeadings(canvas, theme, columns[:], headings[:])
	table := components.BeginTable(canvas, theme, bounds, columns[:])
	table.Header(headings[:])
	for offset, index := range view.visible[start:end] {
		record := &view.rows[index]
		table.Row(hitKnownHostRowList.At(index), record.cells[:], record.selected)
		if box := checks[offset].Rect(); box.Y < bounds.Y+bounds.Height {
			components.BeginCheckbox(canvas, state.Theme(), hitKnownHostCheckList.At(index), ui.Rect{X: box.X, Y: box.Y}, record.selected).Label(record.selectLabel).Paint()
		}
	}
}

func (state *State) paintKnownHostsEmpty(canvas *ui.Canvas, bounds ui.Rect, title, description string, descriptionColor ui.Color) {
	f := state.knownHostsEmptyLayout.Begin(canvas, state.Theme(), bounds)
	f.Column().Fill().Items(flex.Center).Children(func() {
		f.Spacer().HeightPercent(42).Grow(0).MarginSides(0, 0, -16, 0).Shrink(0)
		f.Text(title).FontSize(17).Weight(ui.WeightSemibold).Color(state.pal.text).LineHeight(28).TextCenter().FillWidth()
		f.Text(description).FontSize(metrics.FontSizeS).Color(descriptionColor).LineHeight(24).TextCenter().FillWidth().MarginSides(6, 0, 0, 0)
	})
	f.End()
}

func (state *State) paintKnownHostsDialog(canvas *ui.Canvas, viewport ui.Rect) {
	view, messages := &state.knownHosts, state.messages
	title, confirm := messages.KnownHostsDeleteTitle, messages.KnownHostsDeleteConfirm
	if view.action == knownHostsActionRetrust {
		title, confirm = messages.KnownHostsRetrustTitle, messages.KnownHostsRetrustConfirm
	}
	body := float32(24)
	if view.dialogError != "" {
		body = 72
	}
	dialog := components.BeginDialog(canvas, state.Theme(), viewport, components.DialogOptions{
		Layer: dialogLayer, Scope: dialogScope,
		DismissID: hitKnownHostsDialogDismiss, CloseID: hitKnownHostsDialogClose, InitialFocusID: hitKnownHostsDialogCancel,
		ContentSize: ui.Size{Width: 570}, FitBody: true, BodyHeight: body,
	})
	layout := dialog.Begin(title, view.dialogDescription)
	f := state.partLayout.Begin(canvas, state.Theme(), layout.Body)
	f.Column().Fill().Gap(8).Children(func() {
		f.Text(messages.KnownHostsVerifyWarning).FontSize(metrics.FontSizeS).Weight(ui.WeightMedium).Color(state.pal.muted).LineHeight(24)
		if view.dialogError != "" {
			f.Text(view.dialogError).FontSize(metrics.FontSizeS).Weight(ui.WeightMedium).Color(state.pal.danger).LineHeight(38)
		}
	})
	f.End()
	dialog.Actions(layout,
		components.ButtonAction{ID: hitKnownHostsDialogCancel, Label: messages.Cancel, Variant: components.ButtonSecondary},
		components.ButtonAction{ID: hitKnownHostsDialogConfirm, Label: confirm, Variant: components.ButtonDestructive},
	)
	dialog.End()
}
