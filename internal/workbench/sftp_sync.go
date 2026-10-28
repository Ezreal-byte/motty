package workbench

import (
	"path"
	"strings"

	"motty/internal/sshconn"
	"motty/internal/term"
)

// Reports may arrive before the panel opens or while a listing/editor owns it.
// Consume only the latest one, once, rather than losing it or snapping back on
// every frame after the user has deliberately browsed somewhere else.
func (state *State) terminalDirectory(sessionID uint32, cwd term.Cwd) {
	session, ok := state.sessionByIDPointer(sessionID)
	if !ok || session.term == nil || !session.hasServer() || session.containerCommand != "" || state.sftpFollowOff {
		return
	}
	if session.term.folderIntegrationSeen && !session.term.promptStarted {
		return // a running program or nested login is not this shell's prompt
	}
	session.term.followPath = cwd.Path
}

func (state *State) navigateSFTPDirectory(view *sftpView, directory string) {
	state.requestTerminalDirectory(view, directory)
	if view.terminalPath != "" {
		// A cached listing is optimistic UI, not proof that this directory
		// still exists. Reverse commands require a fresh successful read.
		state.listSFTP(view, directory)
		if !view.busy {
			view.terminalPath = ""
		}
		return
	}
	state.openSFTPDirectory(view, directory)
}

func validSyncDirectory(directory string) bool {
	return path.IsAbs(directory) && strings.IndexFunc(directory, func(r rune) bool { return r < 0x20 || r == 0x7f }) < 0
}

// Only an explicit navigation owns a reverse request. Listing refreshes and
// shell reports use openSFTPDirectory directly and cannot form a feedback loop.
func (state *State) requestTerminalDirectory(view *sftpView, directory string) {
	view.terminalPath, view.terminalSyncSkipped = "", false
	session, ok := state.sftpSession(view)
	if !ok || view.local || view.detached != nil || session.term == nil || session.containerCommand != "" || !session.hasServer() {
		return
	}
	t := session.term
	t.followPath = ""
	if !state.sftpTerminalFollow {
		return
	}
	if t.takeOutput != nil {
		t.takeOutput()
	}
	if !validSyncDirectory(directory) || !state.terminalDirectoryReady(session) {
		view.terminalSyncSkipped = true
		return
	}
	view.terminalPath, view.terminalPromptVersion = path.Clean(directory), t.promptVersion
}

func (state *State) terminalDirectoryReady(session *Session) bool {
	t := session.term
	return session.hasServer() && session.containerCommand == "" && session.phase == sshconn.PhaseConnected &&
		t != nil && t.shell != nil && !t.ended && t.promptReady && !t.promptDirty && t.exec == nil &&
		!t.screen.Modes().AltScreen && !(state.transfer.phase != transferNone && state.transfer.sessionID == session.ID)
}

// The successful listing is the commit point. A failed/obsolete listing must
// never cause a cd, and typing or a new prompt while I/O ran invalidates it.
func (state *State) syncTerminalDirectory(view *sftpView) {
	if view.terminalSyncSkipped {
		view.terminalSyncSkipped = false
		if state.sftpTerminalFollow {
			view.status = state.messages.Tools.FilesTerminalFollowSkipped
		}
	}
	if view.terminalPath == "" || path.Clean(view.path) != view.terminalPath {
		return
	}
	directory, version := view.terminalPath, view.terminalPromptVersion
	view.terminalPath = ""
	if !state.sftpTerminalFollow || view.local || view.detached != nil {
		return
	}
	session, ok := state.sftpSession(view)
	if ok && session.term != nil && session.term.takeOutput != nil {
		session.term.takeOutput()
	}
	if !ok || !state.terminalDirectoryReady(session) || session.term.promptVersion != version ||
		!session.term.send([]byte("builtin cd -- "+shellQuote(directory)+"\r")) {
		view.status = state.messages.Tools.FilesTerminalFollowSkipped
	}
}

func (state *State) followTerminalDirectory(session *Session) {
	t := session.term
	if t == nil || t.followPath == "" {
		return
	}
	if state.sftpFollowOff || !session.hasServer() || session.containerCommand != "" {
		t.followPath = ""
		return
	}
	view := session.files
	if view == nil || view.worker == nil || view.local || view.detached != nil || view.busy ||
		view.dialog != sftpDialogNone || view.editPath != "" || view.pathEditing || view.dropping {
		return
	}
	if state.sftpPathInput && view.pathField.path == view.path && view.pathField.editor.String() != view.path {
		return
	}
	directory := t.followPath
	t.followPath = ""
	if path.Clean(view.path) != path.Clean(directory) {
		state.openSFTPDirectory(view, directory)
	}
}
