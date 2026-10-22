package workbench

import (
	"fmt"
	"path/filepath"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
)

// stateNoticeLines is how many lines of text the dialog has room for.
const stateNoticeLines = 6

// stateNotice is the dialog about the saved state: what the title and the body
// say, built when it is painted, in the language then in use.
type stateNotice struct {
	tooNew      bool
	title, body string
	// file names what the body is about; found and supported are the formats of a
	// file that is too new.
	file, found, supported string
}

// explainStateFile opens the dialog for a state file that was not loaded as it
// was. The status line is not painted anywhere, so this is the only way the user
// learns of it; the dialog has to be dismissed.
func (state *State) explainStateFile(report loadReport) {
	switch report.outcome {
	case loadTooNew:
		state.saveBlocked = true
		state.stateNotice = stateNotice{
			tooNew: true, file: filepath.Base(state.options.StatePath), found: report.found, supported: report.supported,
		}
	case loadUnreadable:
		state.stateNotice = stateNotice{file: filepath.Base(report.kept)}
	default:
		return
	}
	state.modal = modalStateNotice
}

// buildStateNotice writes the dialog's text in the current language.
func (state *State) buildStateNotice() {
	notice, messages := &state.stateNotice, state.messages
	if notice.tooNew {
		notice.title = messages.StateTooNewTitle
		notice.body = fmt.Sprintf(messages.StateTooNewBodyFmt, notice.file, notice.found, notice.supported)
		return
	}
	notice.title = messages.StateUnreadableTitle
	notice.body = fmt.Sprintf(messages.StateUnreadableBodyFmt, notice.file)
}

func (state *State) paintStateNotice(canvas *ui.Canvas, viewport ui.Rect) {
	state.buildStateNotice() // in the current language: it can change while the dialog is up
	dialog := components.BeginDialog(canvas, state.Theme(), viewport, components.DialogOptions{
		Layer: dialogLayer, Scope: dialogScope,
		DismissID: hitStateNoticeDismiss, CloseID: hitStateNoticeDismiss, InitialFocusID: hitStateNoticeOK,
		ContentSize: ui.Size{Width: 560}, FitBody: true, BodyHeight: stateNoticeLines * 21,
	})
	// The dialog's own description is a single line; this text is longer.
	layout := dialog.Begin(state.stateNotice.title, "")
	f := state.chrome.dialog.Begin(canvas, state.Theme(), layout.Body)
	f.Column().Fill().Children(func() {
		f.Paragraph(state.stateNotice.body, stateNoticeLines).FontSize(metrics.FontSizeM).Color(state.pal.muted).LineHeight(21)
	})
	f.End()
	dialog.Actions(layout, components.ButtonAction{ID: hitStateNoticeOK, Label: state.messages.StateNoticeOK})
	dialog.End()
}

func (state *State) clickStateNotice(id ui.ID) {
	if id != hitStateNoticeDismiss && id != hitStateNoticeOK {
		return
	}
	state.closeModal()
	state.showNextPrompt()
}
