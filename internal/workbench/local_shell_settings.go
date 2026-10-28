package workbench

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
)

var (
	hitSettingsLocalShellEdit   = tree.ID(settingsHits, "settings-local-shell-edit")
	hitSettingsLocalShellBrowse = tree.ID(settingsHits, "settings-local-shell-browse")
	hitSettingsLocalShellReset  = tree.ID(settingsHits, "settings-local-shell-reset")
)

func (state *State) paintLocalShellSetting(page *settingsPage) {
	m := state.messages
	description := state.localShell
	if description == "" {
		description = m.SettingsLocalShellDefault
	}
	page.actionRow(m.SettingsLocalShell, description, false,
		components.ButtonAction{ID: hitSettingsLocalShellEdit, Label: m.SettingsLocalShellEdit, Variant: components.ButtonOutline},
		components.ButtonAction{ID: hitSettingsLocalShellBrowse, Label: m.SettingsLocalShellBrowse, Variant: components.ButtonOutline},
		components.ButtonAction{ID: hitSettingsLocalShellReset, Label: m.SettingsLocalShellReset, Variant: components.ButtonGhost, Disabled: state.localShell == ""})
}

func (state *State) editLocalShell() {
	m := state.messages
	form := state.openForm(formDialog{
		title: m.SettingsLocalShell, description: m.SettingsLocalShellDescription, submit: m.SettingsLocalShellSave,
		run: func(state *State, form *formDialog) string {
			return state.chooseLocalShell(form.value(0))
		},
	}, formField{label: m.SettingsLocalShellProgram, placeholder: m.SettingsLocalShellPlaceholder})
	form.set(0, state.localShell)
}

func (state *State) browseLocalShell() {
	paths, err := state.options.OpenFiles(app.OpenFileOptions{Title: state.messages.SettingsLocalShellProgram})
	if errors.Is(err, app.ErrCanceled) || err == nil && len(paths) == 0 {
		return
	}
	if err == nil {
		if message := state.chooseLocalShell(paths[0]); message != "" {
			state.status = message
		}
		return
	}
	state.status = fmt.Sprintf(state.messages.SettingsLocalShellInvalidFmt, err)
}

// chooseLocalShell resolves names from PATH once and keeps an absolute executable
// path. Spaces belong to the path, never to a command line. Existing connections
// retain the executable they started with until the user reconnects them.
func (state *State) chooseLocalShell(value string) string {
	program := strings.TrimSpace(value)
	if len(program) >= 2 && program[0] == '"' && program[len(program)-1] == '"' {
		program = program[1 : len(program)-1] // Windows' Copy as path includes quotes
	}
	if program != "" {
		resolved, err := exec.LookPath(program)
		if err == nil {
			resolved, err = filepath.Abs(resolved)
		}
		if err != nil {
			// The input already shows the path. Keep the inline error short
			// enough to read even when that path is long.
			if cause := errors.Unwrap(err); cause != nil {
				err = cause
			}
			return fmt.Sprintf(state.messages.SettingsLocalShellInvalidFmt, err)
		}
		program = resolved
	}
	if state.localShell != program {
		state.localShell = program
		state.save()
	}
	return ""
}
