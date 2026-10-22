package workbench

import (
	"os"
	"testing"
	"time"

	"motty/internal/autofill"
	"motty/internal/sshconn"
)

type autofillWorld struct {
	state *State
	shell *fakeShell
	clock time.Duration
	path  string
}

// liveAutofillSession is a connected Production tab on a clock the test moves.
func liveAutofillSession(t *testing.T) *autofillWorld {
	t.Helper()
	w := &autofillWorld{}
	world, options := newWorld(t)
	options.Invalidate = func() {}
	options.Now = func() time.Duration { return w.clock }
	w.path = options.StatePath
	w.state = newTestState(t, options)
	fake := connect(t, w.state, world)
	fake.set(func(s *sshconn.Status) { s.Phase = sshconn.PhaseConnected })
	w.state.Wake()
	w.shell = fake.shell(t)
	w.shell.say("\x1b[0m")
	w.state.Wake()
	return w
}

// output is what the remote prints, followed by a pause long enough for the
// prompt to be looked at.
func (w *autofillWorld) output(text string) {
	w.shell.say(text)
	w.state.Wake()
	w.clock += 2 * autofill.Pause
	w.state.Wake()
}

// addRule makes a rule through the dialog, the way the user does.
func (w *autofillWorld) addRule(t *testing.T, name, pattern, scope string, send autofill.Send, fixed string) {
	t.Helper()
	state := w.state
	state.Click(hitSettings)
	state.Click(hitSettingsAutofillAdd)
	typeInto(state, hitAutofillName, name)
	typeInto(state, hitAutofillPattern, pattern)
	if scope != "" {
		typeInto(state, hitAutofillScope, scope)
	}
	switch send {
	case autofill.SendText:
		state.Click(hitAutofillText)
		typeInto(state, hitAutofillFixed, fixed)
	case autofill.SendOTP:
		state.Click(hitAutofillOTP)
	}
	state.Click(hitAutofillSubmit)
	if state.modal != modalNone {
		t.Fatalf("the dialog stayed open: %q", state.autofillRules.dialogError)
	}
}

func TestANewRuleIsOffAndKeepsItsTextOutOfTheSettingsFile(t *testing.T) {
	w := liveAutofillSession(t)
	w.addRule(t, "token", `^Token:`, "", autofill.SendText, "hunter2-fixed")
	rules := w.state.autofillRules.rules
	if len(rules) != 1 || rules[0].Enabled {
		t.Fatalf("rules = %+v", rules)
	}
	if value, found, err := w.state.secrets.vault.Get(autofillSecretName(rules[0])); err != nil || !found || value != "hunter2-fixed" {
		t.Fatalf("vault: %q %v %v", value, found, err)
	}
	data, err := os.ReadFile(w.path)
	if err != nil {
		t.Fatal(err)
	}
	if contains([]string{string(data)}, "hunter2-fixed") || !contains([]string{string(data)}, `"^Token:"`) {
		t.Fatalf("state file: %s", data)
	}
	// Off, it does not answer.
	w.output("Token: ")
	if w.state.modal != modalNone || w.shell.typed() != "" {
		t.Fatalf("a rule that is off answered: modal %d, typed %q", w.state.modal, w.shell.typed())
	}
}

func TestARuleIsRefusedWhenItsPatternCouldNotBeAPrompt(t *testing.T) {
	w := liveAutofillSession(t)
	state := w.state
	state.Click(hitSettings)
	for _, pattern := range []string{"", ".*", "(unclosed"} {
		state.Click(hitSettingsAutofillAdd)
		typeInto(state, hitAutofillName, "bad")
		typeInto(state, hitAutofillPattern, pattern)
		state.Click(hitAutofillSubmit)
		if state.modal != modalAutofillRule || state.autofillRules.dialogError == "" || len(state.autofillRules.rules) != 0 {
			t.Fatalf("%q: modal %d, error %q, rules %d", pattern, state.modal, state.autofillRules.dialogError, len(state.autofillRules.rules))
		}
		state.Click(hitAutofillCancel)
	}
}

func TestARuleAnswersACustomPromptOnceAfterTheUserAllowsIt(t *testing.T) {
	w := liveAutofillSession(t)
	state := w.state
	w.addRule(t, "token", `^Token:`, "Production", autofill.SendText, "s3cret")
	state.Click(hitAutofillToggleList.At(0))
	if !state.autofillRules.rules[0].Enabled {
		t.Fatal("the rule did not turn on")
	}

	w.output("Last login: today\r\nToken: ")
	if state.modal != modalAutofillConfirm || w.shell.typed() != "" {
		t.Fatalf("first use must ask: modal %d, typed %q", state.modal, w.shell.typed())
	}
	state.Click(hitAutofillAllow)
	if got := w.shell.typed(); got != "s3cret\r" {
		t.Fatalf("typed %q", got)
	}

	// A wrong answer brings the same prompt back: it is not answered again.
	w.output("\r\nDenied.\r\nToken: ")
	w.output("")
	if state.modal != modalNone || w.shell.typed() != "s3cret\r" {
		t.Fatalf("the repeated prompt was answered: modal %d, typed %q", state.modal, w.shell.typed())
	}
}

func TestDecliningTheQuestionSendsNothing(t *testing.T) {
	w := liveAutofillSession(t)
	w.addRule(t, "token", `^Token:`, "", autofill.SendText, "s3cret")
	w.state.Click(hitAutofillToggleList.At(0))
	w.output("Token: ")
	w.state.Click(hitAutofillDeny)
	w.output("")
	if w.state.modal != modalNone || w.shell.typed() != "" {
		t.Fatalf("modal %d, typed %q", w.state.modal, w.shell.typed())
	}
}

func TestARuleDoesNotAnswerWhatTheUserTypedOrAFullScreenProgramDrew(t *testing.T) {
	w := liveAutofillSession(t)
	state := w.state
	w.addRule(t, "token", `Token:`, "", autofill.SendText, "s3cret")
	state.Click(hitAutofillToggleList.At(0))

	// The user types the words themselves and the shell echoes them back.
	session := &state.sessions[0]
	state.typed(session, []byte("echo Token:"))
	w.output("$ echo Token:")
	if state.modal != modalNone {
		t.Fatal("the echo of typed text was answered")
	}
	state.typed(session, []byte("\r"))
	w.output("\r\n")

	// An editor on the alternate screen.
	w.output("\x1b[?1049hToken:")
	if state.modal != modalNone || w.shell.typed() != "echo Token:\r" {
		t.Fatalf("modal %d, typed %q", state.modal, w.shell.typed())
	}
}

func TestARuleOnlyAnswersTheHostsItIsScopedTo(t *testing.T) {
	w := liveAutofillSession(t)
	w.addRule(t, "token", `^Token:`, "Staging", autofill.SendText, "s3cret")
	w.state.Click(hitAutofillToggleList.At(0))
	w.output("Token: ")
	if w.state.modal != modalNone {
		t.Fatal("a rule for Staging answered on Production")
	}
}

func TestRulesSurviveARestartWithoutTheirPlainText(t *testing.T) {
	w := liveAutofillSession(t)
	w.addRule(t, "token", `^Token:`, "Servers", autofill.SendOTP, "")
	w.state.Click(hitAutofillToggleList.At(0))
	saved := w.state.savedAutofillRules()
	var restored State
	restored.restoreAutofill(savedState{Autofill: append(saved, savedAutofill{ID: "bad", Name: "x", Pattern: ".*", Send: "text"})})
	if len(restored.autofillRules.rules) != 1 {
		t.Fatalf("restored %d rules", len(restored.autofillRules.rules))
	}
	rule := restored.autofillRules.rules[0]
	if !rule.Enabled || rule.Send != autofill.SendOTP || len(rule.Scope) != 1 || !restored.autofillRules.armed {
		t.Fatalf("rule = %+v", rule)
	}
}
