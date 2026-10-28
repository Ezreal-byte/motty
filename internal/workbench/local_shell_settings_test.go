package workbench

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
	"motty/internal/i18n"
)

func shellExecutable(t *testing.T) string {
	t.Helper()
	name := "sh"
	if runtime.GOOS == "windows" {
		name = "cmd.exe"
	}
	program, err := exec.LookPath(name)
	if err != nil {
		t.Fatal(err)
	}
	return program
}

func TestLocalShellSettingValidatesSavesAndResets(t *testing.T) {
	options := testOptions(t)
	state := newTestState(t, options)
	state.Click(hitSettings)
	state.selectSettingsCategory(settingsTerminal)
	state.Click(hitSettingsLocalShellEdit)
	fillForm(state, filepath.Join(t.TempDir(), "missing-shell"))
	state.Click(hitFormSubmit)
	if state.modal != modalForm || state.form.dialogError == "" || state.localShell != "" {
		t.Fatalf("invalid shell: modal=%v error=%q choice=%q", state.modal, state.form.dialogError, state.localShell)
	}
	program := shellExecutable(t)
	state.form.set(0, `"`+program+`"`)
	state.Click(hitFormSubmit)
	if state.modal != modalNone || state.localShell != program {
		t.Fatalf("valid shell: modal=%v error=%q choice=%q", state.modal, state.form.dialogError, state.localShell)
	}
	state.Close()
	restored := newTestState(t, options)
	if restored.localShell != program {
		t.Fatalf("restart lost shell: %q", restored.localShell)
	}
	restored.Click(hitSettings)
	restored.Click(hitSettingsLocalShellEdit)
	if restored.form.value(0) != program {
		t.Fatal("the editor did not show the saved executable")
	}
	restored.form.set(0, "")
	restored.Click(hitFormCancel)
	if restored.localShell != program {
		t.Fatal("cancel changed the saved shell")
	}
	restored.Click(hitSettingsLocalShellReset)
	if restored.localShell != "" {
		t.Fatal("reset did not restore automatic selection")
	}
	saved, _, err := loadState(options.StatePath)
	if err != nil || saved.LocalShell != "" {
		t.Fatalf("reset was not saved: %q, %v", saved.LocalShell, err)
	}
}

func TestLocalShellPickerAndNameFromPATH(t *testing.T) {
	program := shellExecutable(t)
	options := testOptions(t)
	options.OpenFiles = func(app.OpenFileOptions) ([]string, error) { return []string{program}, nil }
	state := newTestState(t, options)
	state.Click(hitSettings)
	state.Click(hitSettingsLocalShellBrowse)
	if state.localShell != program {
		t.Fatalf("picker chose %q, want %q", state.localShell, program)
	}
	for _, result := range []struct {
		paths []string
		err   error
	}{
		{err: app.ErrCanceled}, {},
		{err: errors.New("picker unavailable")},
		{paths: []string{t.TempDir()}},
	} {
		state.options.OpenFiles = func(app.OpenFileOptions) ([]string, error) { return result.paths, result.err }
		state.Click(hitSettingsLocalShellBrowse)
		if state.localShell != program {
			t.Fatal("a canceled or failed picker replaced the saved shell")
		}
	}
	state.Click(hitSettingsLocalShellEdit)
	state.form.set(0, "   ")
	state.Click(hitFormSubmit)
	if state.localShell != "" || state.modal != modalNone {
		t.Fatal("an empty executable did not restore automatic selection")
	}
	state.Click(hitSettingsLocalShellEdit)
	state.form.set(0, filepath.Base(program))
	state.Click(hitFormSubmit)
	if state.localShell != program || state.modal != modalNone {
		t.Fatalf("name from PATH: %q, %s", state.localShell, state.form.dialogError)
	}
}

func TestLocalShellSettingPaintsAndCanBeFoundInBothLanguages(t *testing.T) {
	for _, language := range []i18n.Language{i18n.English, i18n.Chinese} {
		state := newTestState(t, testOptions(t))
		state.setLanguage(language)
		state.Click(hitSettings)
		state.selectSettingsCategory(settingsTerminal)
		canvas, texts := paintedTexts(state)
		if !contains(texts, state.messages.SettingsLocalShellDefault) {
			t.Fatal("the automatic shell choice is not shown")
		}
		for _, id := range []ui.ID{hitSettingsLocalShellEdit, hitSettingsLocalShellBrowse, hitSettingsLocalShellReset} {
			if _, ok := hitByID(canvas, id); !ok {
				t.Fatalf("missing local shell action %v", id)
			}
		}
		searchSettings(t, state, state.messages.SettingsLocalShell)
		if state.settings.count != 1 || state.settings.results[0].category != settingsTerminal {
			t.Fatalf("shell search returned %d results", state.settings.count)
		}
	}
}

// Exercise settings -> persisted choice -> real local session -> reconnect.
// The executable path has spaces so Windows must preserve it as one program.
func TestConfiguredLocalShellRunsThroughTheWorkbench(t *testing.T) {
	program := shellExecutable(t)
	path := filepath.Join(t.TempDir(), "shell with spaces")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(path, filepath.Base(program))
	data, err := os.ReadFile(program)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o700); err != nil {
		t.Fatal(err)
	}
	state := newTestState(t, testOptions(t))
	state.Click(hitSettings)
	state.Click(hitSettingsLocalShellEdit)
	fillForm(state, path)
	state.Click(hitFormSubmit)
	if state.modal != modalNone {
		t.Fatalf("configure shell: %s", state.form.dialogError)
	}
	state.Click(hitLocalTerminal)
	session, ok := state.sessionByIDPointer(state.activeSessionID)
	if !ok || session.term.shell == nil {
		t.Fatalf("configured shell did not start: %+v", session)
	}
	command, want := "printf 'configured-%s\\n' $((6*7))\n", "configured-42"
	if runtime.GOOS == "windows" {
		command, want = "set /a 6*7\r", "42"
	}
	session.term.send([]byte(command))
	localShellOutput(t, state, session, want)
	first := session.term.shell
	state.Click(hitSettings)
	state.Click(hitSettingsLocalShellReset)
	if session.term.shell != first || session.conn.(*localConn).shell != path {
		t.Fatal("changing the setting replaced an already running shell")
	}
	state.reconnect(session)
	if session.conn.(*localConn).shell != "" || session.term.shell == nil || session.term.shell == first {
		t.Fatal("reconnect did not use the latest shell choice")
	}
}

func TestMissingSavedLocalShellReportsStartupFailure(t *testing.T) {
	options := testOptions(t)
	missing := filepath.Join(t.TempDir(), "removed-shell")
	if err := saveState(options.StatePath, savedState{LocalShell: missing}); err != nil {
		t.Fatal(err)
	}
	state := newTestState(t, options)
	state.Click(hitLocalTerminal)
	session, ok := state.sessionByIDPointer(state.activeSessionID)
	if !ok || !session.shellFailed || session.err == nil || !strings.Contains(session.err.Error(), "removed-shell") {
		t.Fatalf("a missing configured shell did not report failure: %+v", session)
	}
}
