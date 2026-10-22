package workbench

// openEmptyTab adds a tab of one empty pane and shows it: the state a restored or
// split layout can hold. The new-tab button no longer makes one, it asks where to
// connect first.
func openEmptyTab(state *State) {
	workspaceID, _, err := state.deck.Create(state.messages.LabelNewTab)
	if err != nil {
		return
	}
	state.deck.ActivateWorkspace(workspaceID)
	state.activeSessionID = 0
	state.activeSection = sectionTerminal
	state.rebuildTabOptions()
	state.save()
}
