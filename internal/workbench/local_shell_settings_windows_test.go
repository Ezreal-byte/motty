//go:build windows

package workbench

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestConfiguredPowerShellRunsThroughTheWorkbench(t *testing.T) {
	program, err := exec.LookPath("powershell.exe")
	if err != nil {
		t.Skip("Windows PowerShell is not installed")
	}
	// Do not inherit another PowerShell edition's module directory from the
	// test runner. Its PSReadLine may prompt for an untrusted publisher.
	t.Setenv("PSModulePath", filepath.Join(filepath.Dir(program), "Modules"))
	state := newTestState(t, testOptions(t))
	state.Click(hitSettings)
	state.Click(hitSettingsLocalShellEdit)
	fillForm(state, program)
	state.Click(hitFormSubmit)
	state.Click(hitLocalTerminal)
	session, ok := state.sessionByIDPointer(state.activeSessionID)
	if !ok || session.term.shell == nil {
		t.Fatalf("PowerShell did not start: %+v", session)
	}
	session.term.send([]byte("Write-Output ('powershell-' + (6*7))\r"))
	localShellOutput(t, state, session, "powershell-42")
}
