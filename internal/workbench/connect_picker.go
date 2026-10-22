package workbench

import (
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// connectPickerHits is the place of the controls this file paints.
var connectPickerHits = tree.Feature("connect-picker")

// The new-tab picker is the command palette's list showing where a tab can
// connect: this computer, then every saved host. Choosing one creates the tab;
// closing it creates nothing.
var (
	hitConnectPickLocal = tree.ID(connectPickerHits, "connect-pick-local")
	hitConnectPickList  = tree.Sub(connectPickerHits, tree.Name("connect-pick")) // + the host's ID
)

func (state *State) openConnectPicker() {
	state.commandPalette.connect = true
	state.modal = modalCommandPalette
	state.commandPalette.search = text.Editor{}
	state.commandPalette.filtered = false
	state.rebuildCommandPalette()
}

func (state *State) rebuildConnectPicker() {
	palette := &state.commandPalette
	local := localHost(state.messages.LocalTerminalTitle)
	palette.addDetailed(state.messages.LocalTerminal, local.Address, hitConnectPickLocal)
	for _, host := range state.hosts {
		palette.addDetailed(host.Label, state.sessionAddress(host), hitConnectPickList.At(int(host.ID)))
	}
	palette.rawQuery = ""
	palette.query = ""
	palette.filtered = false
	state.refreshCommandPalette()
}

// clickConnectPick connects the picker's choice in a tab of its own.
func (state *State) clickConnectPick(id ui.ID) bool {
	switch {
	case id == hitConnectPickLocal:
		state.pendingWorkspace, state.pendingPane = 0, 0
		state.openLocalTerminal()
	case id.Scope() == hitConnectPickList:
		host, ok := state.hostByID(uint32(item(id)))
		if !ok {
			return true
		}
		state.pendingWorkspace, state.pendingPane = 0, 0
		state.newHostSession(host)
	default:
		return false
	}
	return true
}
