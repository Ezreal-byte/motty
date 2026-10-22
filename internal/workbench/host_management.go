package workbench

import (
	"fmt"
	"strings"

	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

type hostDialogMode uint8

const (
	hostDialogAdd hostDialogMode = iota
	hostDialogEdit
	hostDialogCopy
)

func (state *State) clickHostManagement(id ui.ID) bool {
	if state.importMenu {
		// A choice closes the menu and then runs as if it had been clicked
		// directly; any other press only dismisses it.
		state.importMenu = false
		switch id {
		case hitImportSSHConfig, hitImportHostCatalog, hitImportSessions, hitExportHosts:
		default:
			return true
		}
	}
	if state.hostFilterMenu {
		switch {
		case id == hitHostFilterMenuDismiss || id == hitHostFilterMore:
			state.hostFilterMenu = false
		case id.Scope() == hitHostFilterMenuList:
			state.setHostFilter(true, state.hostGroups[uint32(item(id))].Name)
		default:
			state.hostFilterMenu = false
		}
		return true
	}
	if state.hostGroupMenu {
		switch {
		case id == hitHostGroupDismiss || id == hitMoveHostGroup:
			state.hostGroupMenu = false
		case id == hitHostGroupNone:
			state.hostGroupMenu = false
			state.applyHostGroup("")
		case id == hitHostGroupNew:
			state.hostGroupMenu = false
			state.hostGroupEditor = text.Editor{}
			state.dialogError = ""
			state.modal = modalMoveHostGroup
		case id.Scope() == hitHostGroupItemList:
			group := state.hostGroups[uint32(item(id))].Name
			state.hostGroupMenu = false
			state.applyHostGroup(group)
		default:
			state.hostGroupMenu = false
		}
		return true
	}
	if state.activeSection != sectionHosts || state.modal != modalNone {
		return false
	}
	if hostID, ok := state.hostIDForSelectionHit(id); ok {
		state.selectedHostID = hostID
		state.toggleHostSelection(hostID)
		return true
	}
	if id.Scope() == hitHostGroupRowList {
		state.toggleHostGroup(item(id))
		return true
	}
	if id.Scope() == hitHostGroupOpenList {
		state.toggleHostGroup(item(id))
		return true
	}
	if id.Scope() == hitHostFilterList {
		if index := int(item(id)); index < len(state.hostGroups) {
			state.setHostFilter(true, state.hostGroups[index].Name)
		}
		return true
	}
	switch id {
	case hitHostFilterAll:
		state.setHostFilter(false, "")
	case hitHostFilterMore:
		state.hostFilterMenu = true
	case hitEditHost:
		state.openEditHost()
	case hitCopyHost:
		state.openCopyHost()
	case hitDeleteHost:
		state.openDeleteHosts()
	case hitMoveHostGroup:
		state.captureHostActionIDs()
		state.groupMenu.menu = false
		state.hostGroupMenu = len(state.hostActionIDs) > 0
	case hitImportMenu:
		state.importMenu = true
	case hitExportHosts:
		state.exportHostCatalog()
	case hitImportHostCatalog:
		state.importHostCatalog()
	default:
		return false
	}
	return true
}

func (state *State) hostIDForSelectionHit(id ui.ID) (uint32, bool) {
	if id.Scope() != hitHostCheckList {
		return 0, false
	}
	hostID := uint32(item(id))
	_, ok := state.hostByID(hostID)
	return hostID, ok
}

func (state *State) toggleHostSelection(id uint32) {
	for index, selected := range state.hostSelections {
		if selected != id {
			continue
		}
		copy(state.hostSelections[index:], state.hostSelections[index+1:])
		state.hostSelections = state.hostSelections[:len(state.hostSelections)-1]
		state.refreshHostSelection()
		return
	}
	state.hostSelections = append(state.hostSelections, id)
	state.refreshHostSelection()
}

func (state *State) hostSelected(id uint32) bool {
	for _, selected := range state.hostSelections {
		if selected == id {
			return true
		}
	}
	return false
}

func (state *State) refreshHostSelection() {
	state.hostSelectionText = fmt.Sprintf(state.messages.HostSelectionFmt, len(state.hostSelections))
}

func (state *State) captureHostActionIDs() {
	state.hostActionIDs = state.hostActionIDs[:0]
	for _, id := range state.hostSelections {
		if _, ok := state.hostByID(id); ok {
			state.hostActionIDs = append(state.hostActionIDs, id)
		}
	}
	if len(state.hostActionIDs) == 0 {
		if _, ok := state.hostByID(state.selectedHostID); ok {
			state.hostActionIDs = append(state.hostActionIDs, state.selectedHostID)
		}
	}
}

func (state *State) openEditHost() {
	host, ok := state.singleHostAction()
	if !ok {
		return
	}
	state.openEditHostID(host.ID)
}

func (state *State) openEditHostID(id uint32) {
	host, ok := state.hostByID(id)
	if !ok {
		return
	}
	state.selectedHostID = host.ID
	state.editor.load(host)
	state.prepareOTPState(host)
	state.editor.advancedOpen = state.editor.advancedOpen || state.editor.otpSaved
	state.hostDialogMode = hostDialogEdit
	state.hostDialogID = host.ID
	state.dialogError = ""
	state.modal = modalNewHost
}

func (state *State) openCopyHost() {
	host, ok := state.singleHostAction()
	if !ok {
		return
	}
	state.openCopyHostID(host.ID)
}

func (state *State) openCopyHostID(id uint32) {
	host, ok := state.hostByID(id)
	if !ok {
		return
	}
	state.selectedHostID = host.ID
	state.editor.load(host)
	state.editor.label = text.Editor{}
	state.editor.label.Insert(fmt.Sprintf(state.messages.HostCopyLabelFmt, host.Label))
	state.hostDialogMode = hostDialogCopy
	state.hostDialogID = 0
	state.dialogError = ""
	state.modal = modalNewHost
}

func (state *State) singleHostAction() (Host, bool) {
	if len(state.hostSelections) > 1 {
		return Host{}, false
	}
	if len(state.hostSelections) == 1 {
		return state.hostByID(state.hostSelections[0])
	}
	return state.selectedHost()
}

func (state *State) submitHostDialog() {
	ignoreID := uint32(0)
	if state.hostDialogMode == hostDialogEdit {
		ignoreID = state.hostDialogID
	}
	if message := state.editor.validate(state.hosts, ignoreID, state.messages); message != "" {
		state.dialogError = message
		return
	}
	var previous *Host
	if state.hostDialogMode == hostDialogEdit {
		if index := state.hostIndex(state.hostDialogID); index >= 0 {
			old := state.hosts[index]
			previous = &old
		}
	}
	if message := state.otpPrecheck(previous); message != "" {
		state.dialogError = message
		state.editor.advancedOpen = true
		return
	}
	if state.hostDialogMode == hostDialogEdit {
		state.updateHostFromEditor()
		return
	}
	host := state.editor.host(state.nextHostID)
	state.nextHostID++
	state.hosts = append(state.hosts, host)
	state.selectedHostID = host.ID
	state.modal = modalNone
	state.dialogError = ""
	if state.hostDialogMode == hostDialogCopy {
		state.status = state.messages.HostCopied + host.Label
	} else {
		state.status = state.messages.StatusHostAdded + host.Label
	}
	if state.editor.value(fieldOTP) != "" {
		state.commitOTP(host, nil) // after the host's own line, so the secret's is the one read
	}
	state.hostDataChanged()
	state.save()
	state.showNextPrompt()
}

func (state *State) updateHostFromEditor() {
	index := state.hostIndex(state.hostDialogID)
	if index < 0 {
		state.modal = modalNone
		return
	}
	previous := state.hosts[index]
	updated := state.editor.host(previous.ID)
	updated.CatalogID = previous.CatalogID
	updated.System = previous.System
	state.hosts[index] = updated
	for index := range state.sessions {
		session := &state.sessions[index]
		if session.HostID != updated.ID {
			continue
		}
		session.host = updated
		session.Title = updated.Label
		session.Address = state.sessionAddress(updated)
	}
	for index := range state.forwards {
		forward := &state.forwards[index]
		if !forward.matchesHost(previous) {
			continue
		}
		forward.HostLabel = updated.Label
		forward.HostAddress = updated.Address
		forward.HostUser = updated.User
		forward.HostPort = updated.Port
	}
	state.rebuildTabOptions()
	state.refreshForwards()
	state.refreshProxies()
	state.modal = modalNone
	state.dialogError = ""
	state.status = state.messages.HostUpdated + updated.Label
	state.commitOTP(updated, &previous)
	state.hostDataChanged()
	state.save()
	state.showNextPrompt()
}

func (state *State) hostIndex(id uint32) int {
	for index := range state.hosts {
		if state.hosts[index].ID == id {
			return index
		}
	}
	return -1
}

func (state *State) openDeleteHosts() {
	state.captureHostActionIDs()
	state.openDeleteCapturedHosts()
}

func (state *State) openDeleteCapturedHosts() {
	if len(state.hostActionIDs) == 0 {
		return
	}
	if message := state.hostReferenceError(state.hostActionIDs); message != "" {
		state.status = message
		return
	}
	state.dialogError = ""
	state.hostDialogText = fmt.Sprintf(state.messages.DeleteHostDescriptionFmt, len(state.hostActionIDs))
	state.modal = modalDeleteHost
}

func (state *State) hostReferenceError(ids []uint32) string {
	for _, id := range ids {
		host, ok := state.hostByID(id)
		if !ok {
			continue
		}
		for _, route := range state.proxies {
			if route.TargetID == id || route.JumpID == id {
				return fmt.Sprintf(state.messages.HostDeleteBlockedProxyFmt, host.Label)
			}
		}
		for _, forward := range state.forwards {
			if forward.matchesHost(host) {
				return fmt.Sprintf(state.messages.HostDeleteBlockedForwardFmt, host.Label)
			}
		}
		for _, session := range state.sessions {
			if session.HostID == id {
				return fmt.Sprintf(state.messages.HostDeleteBlockedSessionFmt, host.Label)
			}
		}
	}
	return ""
}

func (state *State) clickDeleteHostDialog(id ui.ID) {
	switch id {
	case hitHostDeleteDismiss, hitHostDeleteClose, hitHostDeleteCancel:
		state.modal = modalNone
		state.dialogError = ""
	case hitHostDeleteConfirm:
		state.confirmDeleteHosts()
	}
}

func (state *State) confirmDeleteHosts() {
	if message := state.hostReferenceError(state.hostActionIDs); message != "" {
		state.dialogError = message
		return
	}
	count := len(state.hostActionIDs)
	kept := state.hosts[:0]
	for _, host := range state.hosts {
		if !containsHostID(state.hostActionIDs, host.ID) {
			kept = append(kept, host)
		}
	}
	clear(state.hosts[len(kept):])
	state.hosts = kept
	state.hostSelections = state.hostSelections[:0]
	state.hostActionIDs = state.hostActionIDs[:0]
	state.modal = modalNone
	state.dialogError = ""
	state.hostDataChanged()
	state.status = fmt.Sprintf(state.messages.HostDeletedFmt, count)
	state.save()
}

func containsHostID(ids []uint32, id uint32) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

func (state *State) clickMoveHostGroupDialog(id ui.ID) {
	switch id {
	case hitMoveGroupDismiss, hitMoveGroupClose, hitMoveGroupCancel:
		state.modal = modalNone
		state.dialogError = ""
	case hitMoveGroupSubmit:
		group := strings.TrimSpace(state.hostGroupEditor.String())
		if group == "" {
			state.dialogError = state.messages.ErrGroupRequired
			return
		}
		state.modal = modalNone
		state.dialogError = ""
		state.applyHostGroup(group)
	}
}

func (state *State) applyHostGroup(group string) {
	count := 0
	for index := range state.hosts {
		if !containsHostID(state.hostActionIDs, state.hosts[index].ID) {
			continue
		}
		state.hosts[index].Group = group
		count++
	}
	if count == 0 {
		return
	}
	state.hostDataChanged()
	label := group
	if label == "" {
		label = state.messages.HostGroupDefault
	}
	state.status = fmt.Sprintf(state.messages.HostMovedFmt, count, label)
	state.save()
}

func (state *State) hostDataChanged() {
	state.visible = nil
	state.refreshHostGroups()
	state.pruneHostSelections()
	selected, ok := state.hostByID(state.selectedHostID)
	if !ok {
		state.selectedHostID = 0
	} else if !state.inHostFilter(selected) {
		// The host just made or moved is not in the group on show: show them all
		// rather than have it vanish.
		state.hostFilterOn = false
		state.rebuildHostTreeRows()
	}
	state.syncVisible()
}

func (state *State) pruneHostSelections() {
	kept := state.hostSelections[:0]
	for _, id := range state.hostSelections {
		if host, ok := state.hostByID(id); ok && state.inHostFilter(host) {
			kept = append(kept, id)
		}
	}
	state.hostSelections = kept
	state.refreshHostSelection()
}

func (state *State) hostManagementKey(event ui.KeyEvent) (ui.EventResult, bool) {
	handled := ui.EventResult{Handled: true, Repaint: true}
	if state.modal == modalDeleteHost {
		if event.Key == ui.KeyEnter && event.Target == hitHostDeleteConfirm && !event.Repeat {
			state.confirmDeleteHosts()
			return handled, true
		}
		return ui.EventResult{}, false
	}
	if state.modal == modalMoveHostGroup {
		if event.Key == ui.KeyEnter && (event.Target == hitMoveGroupField || event.Target == hitMoveGroupSubmit) && !event.Repeat {
			state.clickMoveHostGroupDialog(hitMoveGroupSubmit)
			return handled, true
		}
		return ui.EventResult{}, false
	}
	if state.importMenu && event.Key == ui.KeyEscape {
		state.importMenu = false
		return handled, true
	}
	if state.hostFilterMenu && event.Key == ui.KeyEscape {
		state.hostFilterMenu = false
		return handled, true
	}
	if state.hostGroupMenu && event.Key == ui.KeyEscape {
		state.hostGroupMenu = false
		return handled, true
	}
	if state.activeSection != sectionHosts || state.modal != modalNone || event.Repeat {
		return ui.EventResult{}, false
	}
	hostID, checkbox := state.hostIDForSelectionHit(event.Target)
	if !checkbox {
		if host, ok := state.hostForRow(event.Target); ok {
			hostID = host.ID
		}
	}
	if hostID != 0 && event.Key == ui.KeySpace {
		state.selectedHostID = hostID
		state.toggleHostSelection(hostID)
		return handled, true
	}
	if event.Key == ui.KeyDelete && (hostID != 0 || len(state.hostSelections) > 0) {
		if hostID != 0 {
			state.selectedHostID = hostID
		}
		state.openDeleteHosts()
		return handled, true
	}
	return ui.EventResult{}, false
}
