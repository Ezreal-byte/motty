package workbench

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
)

const (
	// debugPreviewActions is the number of actions in the button group the Debug
	// page shows.
	debugPreviewActions = 3
	// debugSelectOptions is the number of choices in its select.
	debugSelectOptions = 3
)

// debugActionLabels names the actions of the preview group in the active
// language. The array keeps painting allocation-free.
func (state *State) debugActionLabels() [debugPreviewActions]string {
	messages := state.messages
	return [debugPreviewActions]string{messages.DebugAlignLeft, messages.DebugAlignCenter, messages.DebugAlignRight}
}

// debugButtonOptions builds the preview group's stable actions. Viva owns the
// control's look and input; this page only says which actions it holds.
func (state *State) debugButtonOptions() [debugPreviewActions]components.ButtonGroupOption {
	labels := state.debugActionLabels()
	var options [debugPreviewActions]components.ButtonGroupOption
	for index, label := range labels {
		options[index] = components.ButtonGroupOption{
			ID: hitDebugActionList.At(index), Label: label, Variant: components.ButtonOutline,
		}
	}
	return options
}

// debugSelectLabels names the choices of the preview select in the active
// language.
func (state *State) debugSelectLabels() [debugSelectOptions]string {
	messages := state.messages
	return [debugSelectOptions]string{
		messages.DebugEnvironmentDevelopment, messages.DebugEnvironmentStaging, messages.DebugEnvironmentProduction,
	}
}

// clickDebug handles the Debug page's preview controls. The group's hits and the
// select's popup are Viva's; the page only keeps the values and the open state.
func (state *State) clickDebug(id ui.ID) bool {
	if state.modal != modalNone || state.activeSection != sectionDebug {
		return false
	}
	switch {
	case id == hitDebugSelect:
		state.debugSelectOpen = !state.debugSelectOpen
		return true
	case id == hitDebugSelectDismiss:
		state.debugSelectOpen = false
		return true
	}
	if index, ok := id.In(hitDebugActionList); ok && index < debugPreviewActions {
		state.debugAction = index
		return true
	}
	if index, ok := id.In(hitDebugSelectItemList); ok && index < debugSelectOptions {
		state.debugSelectOption = index
		state.debugSelectOpen = false
		return true
	}
	return false
}
