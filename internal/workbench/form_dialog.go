package workbench

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// A form dialog is one modal with up to a few text fields, an error line and a
// submit button, for the actions that only need a password or an address. The
// caller says what the fields are and what submitting does; the dialog does the
// rest, so each such action is a few lines instead of another dialog.

// formDialogHits is the place of the controls this file paints.
var formDialogHits = tree.Feature("form-dialog")

var (
	hitFormFieldList = tree.Sub(formDialogHits, tree.Name("form-field"))
	hitFormSubmit    = tree.ID(formDialogHits, "form-submit")
	hitFormCancel    = tree.ID(formDialogHits, "form-cancel")
	hitFormClose     = tree.ID(formDialogHits, "form-close")
	hitFormDismiss   = tree.ID(formDialogHits, "form-dismiss")
)

const maxFormFields = 5

// formField is one input of a form.
type formField struct {
	label, placeholder string
	secure             bool
	editor             text.Editor
}

// formDialog is the open form. run is called when the user submits: it returns
// the text of an error to show (the form stays open) or "" when the work is done
// or started, which closes the form.
type formDialog struct {
	title, description string
	submit             string
	danger             bool
	fields             [maxFormFields]formField
	count              int
	dialogError        string
	run                func(*State, *formDialog) string
	// cancel, if set, runs instead of closing when the user cancels; a form that
	// stands between two screens uses it to go back.
	cancel func(*State)
}

// openForm shows a form. The fields are copied in; use the returned form to
// preset their text.
func (state *State) openForm(form formDialog, fields ...formField) *formDialog {
	state.formSerial++
	state.form = form
	state.form.count = copy(state.form.fields[:], fields)
	state.form.dialogError = ""
	state.modal = modalForm
	return &state.form
}

// value is the trimmed-free text of field index.
func (form *formDialog) value(index int) string {
	if index < 0 || index >= form.count {
		return ""
	}
	return form.fields[index].editor.String()
}

func (form *formDialog) set(index int, value string) {
	form.fields[index].editor = text.Editor{}
	form.fields[index].editor.Insert(value)
}

func (state *State) formEditor(hit ui.ID) *text.Editor {
	if hit.Scope() != hitFormFieldList {
		return nil
	}
	return &state.form.fields[uint32(item(hit))].editor
}

func (state *State) clickForm(id ui.ID) {
	switch id {
	case hitFormSubmit:
		state.submitForm()
	case hitFormCancel, hitFormClose, hitFormDismiss:
		cancel := state.form.cancel
		state.closeModal()
		if cancel != nil {
			cancel(state)
		}
	}
}

func (state *State) submitForm() {
	form := &state.form
	if form.run == nil {
		state.closeModal()
		return
	}
	serial := state.formSerial
	message := form.run(state, form)
	if state.formSerial != serial || state.modal != modalForm {
		return // run opened another form or dialog; that one stands
	}
	if message != "" {
		form.dialogError = message
		return
	}
	state.closeModal()
}

// bodyHeight is the dialog body's height: its fields, and the error under them
// when there is one.
func (form *formDialog) bodyHeight() float32 {
	return withError(float32(form.count)*66, form.dialogError)
}

func (state *State) paintFormDialog(canvas *ui.Canvas, viewport ui.Rect) {
	theme := state.Theme()
	form := &state.form
	initial := hitFormSubmit
	if form.count > 0 {
		initial = hitFormFieldList.At(0)
	}
	dialog := components.BeginDialog(canvas, theme, viewport, components.DialogOptions{
		Layer: dialogLayer, Scope: dialogScope,
		DismissID: hitFormDismiss, CloseID: hitFormClose, InitialFocusID: initial,
		ContentSize: ui.Size{Width: 540}, FitBody: true, BodyHeight: form.bodyHeight(),
	})
	layout := dialog.Begin(form.title, form.description)
	var slots [maxFormFields]flex.Box
	f := state.settingsRow.Begin(canvas, theme, layout.Body)
	f.Column().Fill().PadSides(4, 4, 0, 4).Children(func() {
		for index := 0; index < form.count; index++ {
			slots[index] = f.Slot().Height(62).MarginSides(0, 0, 4, 0)
		}
		state.problemLine(f, form.dialogError)
	})
	f.End()
	for index := 0; index < form.count; index++ {
		field := &form.fields[index]
		id := hitFormFieldList.At(index)
		content := components.BeginField(canvas, theme, slots[index].Rect()).Paint(field.label, "", "")
		value := field.editor.String()
		focused := canvas.Focused() == id
		components.PaintInput(canvas, theme, content, components.InputSnapshot{
			ID: id, Label: field.label, Value: value, Placeholder: field.placeholder, Secure: field.secure,
			Focused: focused, PlatformEditing: focused && value != "",
		})
	}
	variant := components.ButtonDefault
	if form.danger {
		variant = components.ButtonDestructive
	}
	dialog.Actions(layout,
		components.ButtonAction{ID: hitFormCancel, Label: state.messages.Cancel, Variant: components.ButtonSecondary},
		components.ButtonAction{ID: hitFormSubmit, Label: form.submit, Variant: variant},
	)
	dialog.End()
}
