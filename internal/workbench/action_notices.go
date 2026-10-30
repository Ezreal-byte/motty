package workbench

import "github.com/vivaui/viva/components"

// Explicit actions need feedback on pages that do not display state.status.
// Share SFTP's bounded, deduplicated Viva Toast queue and request a repaint for
// keyboard and command-palette actions as well as pointer clicks.
func (state *State) showActionWarning(title, detail string) {
	state.showActionNotice(title, detail, components.ToastWarning)
}

func (state *State) showActionError(title, detail string) {
	state.showActionNotice(title, detail, components.ToastError)
}

func (state *State) showActionInfo(title, detail string) {
	state.showActionNotice(title, detail, components.ToastInfo)
}

func (state *State) showActionNotice(title, detail string, kind components.ToastType) {
	state.status = detail
	state.fileNotices.show(title, detail, kind)
	if state.options.Invalidate != nil {
		state.options.Invalidate()
	}
}
