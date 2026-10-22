package workbench

import (
	"fmt"
	"strings"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// Snippets are commands the user keeps to send to a terminal later: from the
// command palette, as if typed. They live in the state file, and are managed in
// Settings.

type snippet struct {
	Name    string
	Command string
	// Run presses Enter after sending, so the command runs at once.
	Run bool
}

const (
	maxSnippets       = 48
	maxSnippetName    = 80
	maxSnippetCommand = 1000
)

// snippetsHits is the place of the controls this file paints.
var snippetsHits = tree.Feature("snippets")

var (
	hitSettingsSnippetAdd = tree.ID(snippetsHits, "settings-snippet-add")
	hitSnippetName        = tree.ID(snippetsHits, "snippet-name")
	hitSnippetCommand     = tree.ID(snippetsHits, "snippet-command")
	hitSnippetRun         = tree.ID(snippetsHits, "snippet-run")
	hitSnippetSubmit      = tree.ID(snippetsHits, "snippet-submit")
	hitSnippetCancel      = tree.ID(snippetsHits, "snippet-cancel")
	hitSnippetClose       = tree.ID(snippetsHits, "snippet-close")
	hitSnippetDismiss     = tree.ID(snippetsHits, "snippet-dismiss")
)

var (
	hitSnippetEditList   = tree.Sub(snippetsHits, tree.Name("snippet-edit")) // + snippet index
	hitSnippetDeleteList = tree.Sub(snippetsHits, tree.Name("snippet-delete"))
	hitSnippetSendList   = tree.Sub(snippetsHits, tree.Name("snippet-send")) // the command palette's entries
)

type snippetsUI struct {
	list []snippet

	// The dialog: which snippet it edits (-1 adds one) and the fields.
	editing     int
	name        text.Editor
	command     text.Editor
	run         bool
	dialogError string
}

// cleanSnippetText makes one line of what was typed: a snippet is sent as
// keystrokes, so a control character in it would be a key the user never meant.
func cleanSnippetText(value string, limit int) string {
	value = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, value)
	value = strings.TrimSpace(value)
	if len(value) > limit {
		value = strings.ToValidUTF8(value[:limit], "")
	}
	return value
}

func (state *State) openSnippetDialog(index int) {
	form := &state.snippets
	form.editing = index
	form.name, form.command = text.Editor{}, text.Editor{}
	form.run = true
	form.dialogError = ""
	if index >= 0 && index < len(form.list) {
		form.name.Insert(form.list[index].Name)
		form.command.Insert(form.list[index].Command)
		form.run = form.list[index].Run
	}
	state.modal = modalSnippet
}

func (state *State) clickSnippetDialog(id ui.ID) {
	form := &state.snippets
	switch id {
	case hitSnippetSubmit:
		state.submitSnippet()
	case hitSnippetCancel, hitSnippetClose, hitSnippetDismiss:
		state.closeModal()
	case hitSnippetRun:
		form.run = !form.run
	}
}

func (state *State) submitSnippet() {
	form, m := &state.snippets, state.messages.Tools
	entry := snippet{
		Name:    cleanSnippetText(form.name.String(), maxSnippetName),
		Command: cleanSnippetText(form.command.String(), maxSnippetCommand),
		Run:     form.run,
	}
	switch {
	case entry.Name == "":
		form.dialogError = m.SnippetErrorName
		return
	case entry.Command == "":
		form.dialogError = m.SnippetErrorCommand
		return
	}
	if form.editing >= 0 && form.editing < len(form.list) {
		form.list[form.editing] = entry
	} else if len(form.list) >= maxSnippets {
		form.dialogError = m.SnippetErrorTooMany
		return
	} else {
		form.list = append(form.list, entry)
	}
	state.closeModal()
	state.save()
}

func (state *State) deleteSnippet(index int) {
	list := state.snippets.list
	if index < 0 || index >= len(list) {
		return
	}
	state.snippets.list = append(list[:index:index], list[index+1:]...)
	state.save()
}

// sendSnippet types a snippet into the active terminal, and into the other panes
// of its tab while the tab is broadcasting.
func (state *State) sendSnippet(index int) {
	m := state.messages.Tools
	if index < 0 || index >= len(state.snippets.list) {
		return
	}
	entry := state.snippets.list[index]
	session, ok := state.sessionByIDPointer(state.activeSessionID)
	if !ok || session.term == nil {
		state.status = m.StatusSnippetNoTerminal
		return
	}
	input := make([]byte, 0, len(entry.Command)+1)
	input = append(input, entry.Command...)
	if entry.Run {
		input = append(input, '\r')
	}
	if !state.typed(session, input) {
		state.status = m.StatusSnippetNotConnected
		return
	}
	state.status = fmt.Sprintf(m.StatusSnippetSentFmt, entry.Name)
}

func (state *State) paintSnippetDialog(canvas *ui.Canvas, viewport ui.Rect) {
	m, theme := state.messages.Tools, state.Theme()
	form := &state.snippets
	title := m.SnippetDialogAddTitle
	if form.editing >= 0 {
		title = m.SnippetDialogEditTitle
	}
	dialog := components.BeginDialog(canvas, theme, viewport, components.DialogOptions{
		Layer: dialogLayer, Scope: dialogScope,
		DismissID: hitSnippetDismiss, CloseID: hitSnippetClose, InitialFocusID: hitSnippetName,
		ContentSize: ui.Size{Width: 560}, FitBody: true, BodyHeight: withError(170, form.dialogError), // two fields and a box
	})
	layout := dialog.Begin(title, "")
	entries := [...]struct {
		id          ui.ID
		label       string
		placeholder string
		value       string
	}{
		{hitSnippetName, m.SnippetName, m.SnippetNamePlaceholder, form.name.String()},
		{hitSnippetCommand, m.SnippetCommand, m.SnippetCommandPlaceholder, form.command.String()},
	}
	var fields [len(entries)]flex.Box
	var run flex.Box
	f := state.settingsRow.Begin(canvas, theme, layout.Body)
	f.Column().Fill().PadSides(2, 4, 0, 4).Children(func() {
		for index := range fields {
			fields[index] = f.Slot().Height(62).MarginSides(0, 0, 6, 0)
		}
		run = state.declareDialogCheckbox(f, m.SnippetRun, 7, 0)
		if form.dialogError != "" {
			f.Text(form.dialogError).FontSize(metrics.FontSizeS).Weight(ui.WeightMedium).Color(state.pal.danger).LineHeight(18).MarginSides(15, 0, 0, 0)
		}
	})
	f.End()
	for index, entry := range entries {
		content := components.BeginField(canvas, theme, fields[index].Rect()).Paint(entry.label, "", "")
		focused := canvas.Focused() == entry.id
		components.PaintInput(canvas, theme, content, components.InputSnapshot{
			ID: entry.id, Label: entry.label, Value: entry.value, Placeholder: entry.placeholder,
			Focused: focused, PlatformEditing: focused && entry.value != "", Invalid: form.dialogError != "",
		})
	}
	state.paintDialogCheckbox(canvas, hitSnippetRun, m.SnippetRun, form.run, run)
	dialog.Actions(layout,
		components.ButtonAction{ID: hitSnippetCancel, Label: state.messages.Cancel, Variant: components.ButtonSecondary},
		components.ButtonAction{ID: hitSnippetSubmit, Label: m.SnippetSave},
	)
	dialog.End()
}

// paintSnippetSettings is the Snippets group of Settings.
func (state *State) paintSnippetSettings(page *settingsPage) {
	m := state.messages.Tools
	list := state.snippets.list
	page.group(m.SnippetsGroup, func() {
		for index, entry := range list {
			command := entry.Command
			if entry.Run {
				command = "⏎ " + command
			}
			page.actionRow(entry.Name, command, false,
				components.ButtonAction{ID: hitSnippetEditList.At(index), Label: m.SnippetEdit, Variant: components.ButtonOutline},
				components.ButtonAction{ID: hitSnippetDeleteList.At(index), Label: m.SnippetDelete, Variant: components.ButtonOutline})
		}
		page.actionRow(m.SnippetsRow, m.SnippetsDescription, len(list) >= maxSnippets,
			components.ButtonAction{ID: hitSettingsSnippetAdd, Label: m.SnippetAdd, Variant: components.ButtonOutline})
	})
}
