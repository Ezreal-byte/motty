package workbench

import (
	"fmt"
	"strings"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
	"motty/internal/update"
)

const (
	// updateNotesMaxHeight is the most room the release notes take; longer notes
	// scroll inside it, so fifty entries do not fill the window.
	updateNotesMaxHeight float32 = 9 * updateNotesLine
	updateNotesLine      float32 = 20
	// updateNotesGutter keeps text clear of the scrollbar (6 wide) once there is one.
	updateNotesGutter float32 = 14
	// The notes sit in a bordered box; these are the room between its edge and
	// the text, and the right edge is nearer so the scrollbar hugs the border.
	updateNotesPadX     float32 = 12
	updateNotesPadY     float32 = 8
	updateNotesPadRight float32 = 4
	updateWhyLines              = 2
)

var hitUpdateNotes = tree.ID(launchHits, "update-notes")

// updateNotes is the release notes as the dialog shows them: what was measured
// for the width it has, and where the text is scrolled to.
type updateNotes struct {
	text string // the notes as plain lines; empty when the release has none

	measuredText  string
	measuredWidth float32
	lines         int
	scrolls       bool

	scroll float32
	// viewHeight, contentHeight and max are as of the last paint, which the
	// wheel and the keys act on.
	viewHeight, contentHeight, max float32
}

// wrappedLines is how many lines text takes in width, counting a blank line as one.
func wrappedLines(canvas *ui.Canvas, text string, size, width float32) int {
	total := 0
	var scratch []string
	for _, line := range strings.Split(text, "\n") {
		scratch = scratch[:0]
		if line != "" {
			scratch = canvas.WrapLines(scratch, line, size, ui.WeightRegular, width, 1<<12)
		}
		total += max(len(scratch), 1)
	}
	return total
}

// measure counts the lines of text in the width the dialog gives it, and whether
// they are more than fit, in which case the scrollbar takes some of that width.
func (notes *updateNotes) measure(canvas *ui.Canvas, text string, width float32) {
	if notes.lines > 0 && notes.measuredWidth == width && notes.measuredText == text {
		return
	}
	notes.measuredText, notes.measuredWidth = text, width
	notes.lines = wrappedLines(canvas, text, metrics.FontSizeS, width)
	notes.scrolls = float32(notes.lines)*updateNotesLine > updateNotesMaxHeight
	if notes.scrolls {
		notes.lines = wrappedLines(canvas, text, metrics.FontSizeS, width-updateNotesGutter)
	}
}

func (state *State) paintUpdateDialog(canvas *ui.Canvas, viewport ui.Rect) {
	m := state.messages.Tools
	notes := &state.update.notes
	install := state.canInstallUpdate()
	why := state.whyNoInstall()
	primary := components.ButtonAction{ID: hitUpdateDownload, Label: m.GeneralUpdateDownload}
	description := fmt.Sprintf(m.UpdateDialogDescriptionFmt, update.Current)
	if install {
		primary = components.ButtonAction{ID: hitUpdateInstall, Label: m.UpdateInstall}
		description = fmt.Sprintf(m.UpdateInstallDescriptionFmt, update.Current)
	}

	// The body is as tall as what it holds, up to the cap, so a short note does
	// not leave a gap above the buttons.
	const contentWidth float32 = 560
	bodyWidth := contentWidth - 2*(metrics.PadSurface+metrics.GapItem)
	viewWidth := bodyWidth - updateNotesPadX - updateNotesPadRight
	text := notes.text
	if text == "" {
		text = m.UpdateNotesEmpty
	}
	notes.measure(canvas, text, viewWidth)
	notes.contentHeight = float32(notes.lines) * updateNotesLine
	notes.viewHeight = min(notes.contentHeight, updateNotesMaxHeight)
	boxHeight := notes.viewHeight + 2*updateNotesPadY
	whyHeight := float32(0)
	if why != "" {
		whyLines := min(wrappedLines(canvas, why, metrics.FontSizeS, bodyWidth), updateWhyLines)
		whyHeight = 12 + float32(whyLines)*updateNotesLine
	}

	dialog := components.BeginDialog(canvas, state.Theme(), viewport, components.DialogOptions{
		Layer: dialogLayer, Scope: dialogScope,
		DismissID: hitUpdateDismiss, CloseID: hitUpdateClose, InitialFocusID: primary.ID,
		ContentSize: ui.Size{Width: contentWidth}, FitBody: true, BodyHeight: boxHeight + whyHeight,
	})
	layout := dialog.Begin(fmt.Sprintf(m.UpdateDialogTitleFmt, state.update.release.Version), description)

	box := ui.Rect{X: layout.Body.X, Y: layout.Body.Y, Width: layout.Body.Width, Height: boxHeight}
	canvas.RoundedBorder(box, radius.control, 1, state.pal.line, state.pal.surface)
	view := ui.Rect{X: box.X + updateNotesPadX, Y: box.Y + updateNotesPadY, Width: box.Width - updateNotesPadX - updateNotesPadRight, Height: notes.viewHeight}
	if notes.scrolls {
		canvas.HitWithOptions(hitUpdateNotes, view, m.UpdateNotesLabel, ui.HitOptions{Role: ui.RoleGroup})
	}
	scroller := components.BeginMessageScroller(canvas, state.Theme(), view, notes.contentHeight, notes.scroll)
	geometry := scroller.BeginContent()
	textWidth := view.Width
	if notes.scrolls {
		textWidth -= updateNotesGutter
	}
	f := state.settingsRow.Begin(canvas, state.Theme(), ui.Rect{X: geometry.Content.X, Y: geometry.Content.Y, Width: textWidth, Height: notes.contentHeight})
	f.Column().Fill().Children(func() {
		f.Wrap(text).FontSize(metrics.FontSizeS).Color(state.pal.muted).LineHeight(updateNotesLine)
	})
	f.End()
	geometry = scroller.EndContent()
	notes.scroll, notes.max = geometry.Offset, geometry.MaxOffset

	if why != "" {
		f := state.partLayout.Begin(canvas, state.Theme(), ui.Rect{X: box.X, Y: box.Y + box.Height + 12, Width: box.Width, Height: whyHeight - 12})
		f.Paragraph(why, updateWhyLines).FontSize(metrics.FontSizeS).Color(state.pal.text).LineHeight(updateNotesLine)
		f.End()
	}
	dialog.Actions(layout,
		components.ButtonAction{ID: hitUpdateSkip, Label: m.UpdateSkip, Variant: components.ButtonOutline},
		components.ButtonAction{ID: hitUpdateLater, Label: m.UpdateLater, Variant: components.ButtonSecondary},
		primary,
	)
	dialog.End()
}

// scrollUpdateNotes is the wheel over the update dialog: the notes move, and
// nothing behind the dialog does.
func (state *State) scrollUpdateNotes(event ui.ScrollEvent) ui.EventResult {
	notes := &state.update.notes
	step := event.DeltaY
	if !event.Precise && step > 0 && step < updateNotesLine {
		step = updateNotesLine
	} else if !event.Precise && step < 0 && step > -updateNotesLine {
		step = -updateNotesLine
	}
	before := notes.scroll
	notes.scroll = min(max(before+step, 0), notes.max)
	return ui.EventResult{Handled: true, Repaint: notes.scroll != before}
}

func (state *State) updateKey(event ui.KeyEvent) (ui.EventResult, bool) {
	if state.modal != modalUpdate || event.Repeat && event.Key == ui.KeyEnter {
		return ui.EventResult{}, false
	}
	notes := &state.update.notes
	if event.Target == hitUpdateNotes {
		if offset, ok := notes.offsetForKey(event.Key); ok {
			notes.scroll = offset
			return ui.EventResult{Handled: true, Repaint: true}, true
		}
		return ui.EventResult{}, false
	}
	if event.Key != ui.KeyEnter {
		return ui.EventResult{}, false
	}
	switch event.Target {
	case hitUpdateInstall, hitUpdateDownload, hitUpdateSkip, hitUpdateLater:
		state.clickUpdateDialog(event.Target)
		return ui.EventResult{Handled: true, Repaint: true}, true
	}
	return ui.EventResult{}, false
}

// offsetForKey is where the keyboard moves the notes to: a line or two for the
// arrows, a screen for the page keys, the ends for Home and End.
func (notes *updateNotes) offsetForKey(key ui.Key) (float32, bool) {
	step := 2 * updateNotesLine
	page := max(notes.viewHeight*0.9, step)
	offset := notes.scroll
	switch key {
	case ui.KeyUp:
		offset -= step
	case ui.KeyDown:
		offset += step
	case ui.KeyPageUp:
		offset -= page
	case ui.KeyPageDown:
		offset += page
	case ui.KeyHome:
		offset = 0
	case ui.KeyEnd:
		offset = notes.max
	default:
		return offset, false
	}
	return min(max(offset, 0), notes.max), true
}
