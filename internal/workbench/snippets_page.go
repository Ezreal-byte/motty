package workbench

import (
	"fmt"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// refreshSnippets makes what the page shows of each snippet and the count under
// it, after the list or the language changed.
func (state *State) refreshSnippets() {
	form := &state.snippets
	for index := range form.list {
		entry := &form.list[index]
		command := entry.Command
		if entry.Run {
			command = "⏎ " + command
		}
		shortcut := ""
		if entry.Shortcut.valid() {
			shortcut = entry.Shortcut.label()
		}
		entry.cells = [3]string{entry.Name, shortcut, command}
	}
	form.selected = max(0, min(form.selected, len(form.list)-1))
	form.countText = fmt.Sprintf(state.messages.Tools.SnippetCountFmt, len(form.list))
}

func (state *State) clickSnippetsPage(id ui.ID) bool {
	if state.activeSection != sectionSnippets || state.modal != modalNone {
		return false
	}
	switch {
	case id == hitSnippetAdd:
		state.openSnippetDialog(-1)
	case id == hitSnippetEdit:
		state.openSnippetDialog(state.snippets.selected)
	case id == hitSnippetRemove:
		state.deleteSnippet(state.snippets.selected)
	default:
		return false
	}
	return true
}

// paintSnippetsWorkspace is the Snippets page: the commands kept to send to a
// terminal, with the shortcut of each, and the way to add, edit and remove them.
func (state *State) paintSnippetsWorkspace(canvas *ui.Canvas, bounds ui.Rect) {
	pal, m := state.pal, state.messages.Tools
	form := &state.snippets
	canAdd := len(form.list) < maxSnippets
	hasSelection := len(form.list) > 0
	// The title keeps a readable column beside the buttons; a card too narrow for
	// them shows the buttons as their icons, whose tooltips name them.
	const (
		titleLeast = float32(220)
		gap        = float32(10)
	)
	labeled := toolbarButtonWidth(canvas, m.SnippetDelete, false) + toolbarButtonWidth(canvas, m.SnippetEdit, false) +
		toolbarButtonWidth(canvas, m.SnippetAdd, true) + 2*gap + gap + titleLeast
	condensed := labeled > bounds.Width-36
	removeWidth, removeIcon := toolbarButtonLayout(canvas, m.SnippetDelete, 0, iconTrash, condensed)
	editWidth, editIcon := toolbarButtonLayout(canvas, m.SnippetEdit, 0, iconPencil, condensed)
	addWidth, addIcon := toolbarButtonLayout(canvas, m.SnippetAdd, iconPlus, iconPlus, condensed)

	var content, emptyIcon, emptyAdd, remove, edit, add flex.Box
	f := form.layout.Begin(canvas, state.Theme(), bounds)
	f.Column().Fill().PadSides(12, 18, 0, 18).Children(func() {
		f.Row().MinHeight(58).Gap(gap).MarginSides(0, 0, 12, 0).Items(flex.Start).Children(func() {
			f.Column().Grow(1).Gap(3).MarginSides(4, 0, 0, 0).Children(func() {
				f.Text(m.SnippetsTitle).FontSize(18).Weight(ui.WeightSemibold).Color(pal.text).Ellipsis()
				f.Paragraph(m.SnippetsPageDescription, 2).FontSize(metrics.FontSizeS).Color(pal.muted)
			})
			remove = f.Slot().Size(removeWidth, toolbarHeight)
			edit = f.Slot().Size(editWidth, toolbarHeight)
			add = f.Slot().Size(addWidth, toolbarHeight)
		})
		if len(form.list) == 0 {
			emptyIcon, emptyAdd = state.declareEmptyPage(f, bounds, m.SnippetEmpty, m.SnippetEmptySub, m.SnippetAdd)
		} else {
			content = f.Slot().Grow(1).MarginSides(0, 0, 9, 0)
		}
		f.Divider().Background(pal.line)
		f.Row().Height(50).PadSides(8, 0, 20, 0).Items(flex.Center).Children(func() {
			f.Text(form.countText).FontSize(metrics.FontSizeS).Weight(ui.WeightMedium).Color(pal.faint).Width(162)
			if hasSelection {
				f.Text(form.list[form.selected].Name).FontSize(metrics.FontSizeS).Color(pal.muted).Grow(1).Ellipsis()
			}
		})
	})
	f.End()

	state.paintToolbarButton(canvas, hitSnippetRemove, remove.Rect().X, remove.Rect().Y, removeWidth, m.SnippetDelete, removeIcon, false, hasSelection, condensed)
	state.paintToolbarButton(canvas, hitSnippetEdit, edit.Rect().X, edit.Rect().Y, editWidth, m.SnippetEdit, editIcon, false, hasSelection, condensed)
	state.paintToolbarButton(canvas, hitSnippetAdd, add.Rect().X, add.Rect().Y, addWidth, m.SnippetAdd, addIcon, true, canAdd, condensed)

	if len(form.list) == 0 {
		state.paintEmptyPage(canvas, emptyIcon, emptyAdd, iconNote, hitSnippetAdd, m.SnippetAdd, canAdd)
		return
	}
	columns := [...]components.TableColumn{{Width: 220}, {Width: 170}, {Width: 0}}
	headings := [...]string{m.SnippetColumnName, m.SnippetColumnShortcut, m.SnippetColumnCommand}
	components.FitTableHeadings(canvas, state.Theme(), columns[:], headings[:])
	table := components.BeginTable(canvas, state.Theme(), content.Rect(), columns[:])
	table.Header(headings[:])
	for index := range form.list {
		if table.RowAt(hitSnippetRowList, index, form.list[index].cells[:], index == form.selected) && state.modal == modalNone {
			form.selected = index
		}
	}
}
