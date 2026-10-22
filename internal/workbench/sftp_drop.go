package workbench

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/vivaui/viva/ui"
)

// Files dragged in from the system. Over the file panel they are uploaded: onto a
// folder row into that folder, anywhere else on the panel into the folder on
// show. Over the terminal of a shell on this computer they are typed as paths,
// quoted, as other terminals do. Nowhere else takes them.

// pointInContext reports whether a window point is on the side panel.
func (state *State) pointInContext(x, y float32) bool {
	return state.contextRect.Width > 0 && pointInRect(x, y, state.contextRect)
}

// dropPanel is the file panel of the active session when it can take an upload
// at the point: it shows the server's files, is not busy with a dialog or the
// editor, and the point is on it.
func (state *State) dropPanel(x, y float32) (*Session, *sftpView) {
	if state.activeSection != sectionTerminal || state.modal != modalNone || !state.pointInContext(x, y) {
		return nil, nil
	}
	session, ok := state.sessionByIDPointer(state.activeSessionID)
	if !ok || session.files == nil || session.conn == nil {
		return nil, nil
	}
	view := session.files
	if view.local || view.dialog != sftpDialogNone || view.editPath != "" || view.path == "" {
		return nil, nil
	}
	return session, view
}

// dropFolderRow is the row of a folder under the pointer, or -1.
func (state *State) dropFolderRow(view *sftpView, target ui.ID) int {
	local, ok := tree.Local(contextMount, target)
	if !ok || local.Scope() != hitSFTPEntryList {
		return -1
	}
	index := view.first + item(local)
	if index < 0 || index >= len(view.rows) || !view.rows[index].entry.IsDir() {
		return -1
	}
	return index
}

// FileDrop receives files dragged from the system over the window.
func (state *State) FileDrop(event ui.FileDropEvent) ui.EventResult {
	switch event.Phase {
	case ui.FileDropLeave:
		return ui.EventResult{Repaint: state.clearDrop()}
	case ui.FileDropOver:
		if _, view := state.dropPanel(event.X, event.Y); view != nil {
			row := state.dropFolderRow(view, event.Target)
			changed := !view.dropping || view.dropRow != row
			view.dropping, view.dropRow = true, row
			return ui.EventResult{Handled: true, Repaint: changed || state.clearDropExcept(view)}
		}
		repaint := state.clearDrop()
		return ui.EventResult{Handled: state.terminalTakesPaths(event.X, event.Y), Repaint: repaint}
	}
	repaint := state.clearDrop()
	if session, view := state.dropPanel(event.X, event.Y); view != nil {
		remoteDir := view.path
		if row := state.dropFolderRow(view, event.Target); row >= 0 {
			remoteDir = path.Join(view.path, view.rows[row].entry.Name)
		}
		queued := 0
		for _, local := range event.Paths {
			info, err := os.Stat(local)
			if err != nil {
				continue
			}
			state.queueSFTPUploadTo(view, local, info.IsDir(), remoteDir)
			queued++
		}
		if queued > 0 {
			view.status = fmt.Sprintf(state.messages.Tools.DropUploadingFmt, queued, remoteDir)
		}
		_ = session
		return ui.EventResult{Handled: queued > 0, Repaint: true}
	}
	if state.terminalTakesPaths(event.X, event.Y) {
		session, _ := state.sessionByIDPointer(state.activeSessionID)
		session.term.send([]byte(quotedPaths(event.Paths) + " "))
		return ui.EventResult{Handled: true, Repaint: true}
	}
	return ui.EventResult{Repaint: repaint}
}

// terminalTakesPaths reports whether the point is on the terminal of a live
// shell on this computer, which can be handed paths to type.
func (state *State) terminalTakesPaths(x, y float32) bool {
	if state.activeSection != sectionTerminal || state.modal != modalNone || state.pointInContext(x, y) {
		return false
	}
	session, ok := state.sessionByIDPointer(state.activeSessionID)
	return ok && session.isLocal() && session.term != nil && session.term.shell != nil
}

// clearDrop ends a hover over the panel and reports whether anything showed it.
func (state *State) clearDrop() bool {
	changed := false
	for index := range state.sessions {
		if view := state.sessions[index].files; view != nil && view.dropping {
			view.dropping, view.dropRow = false, -1
			changed = true
		}
	}
	return changed
}

// clearDropExcept clears the hover of every panel but view's.
func (state *State) clearDropExcept(keep *sftpView) bool {
	changed := false
	for index := range state.sessions {
		if view := state.sessions[index].files; view != nil && view != keep && view.dropping {
			view.dropping, view.dropRow = false, -1
			changed = true
		}
	}
	return changed
}

// quotedPaths makes paths safe to type into the shell of this computer.
func quotedPaths(paths []string) string {
	quoted := make([]string, len(paths))
	for index, p := range paths {
		if runtime.GOOS == "windows" {
			quoted[index] = `"` + strings.ReplaceAll(p, `"`, `""`) + `"`
		} else {
			quoted[index] = shellQuote(p)
		}
	}
	return strings.Join(quoted, " ")
}
