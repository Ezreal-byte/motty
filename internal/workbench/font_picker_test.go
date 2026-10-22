package workbench

import (
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/ui"
)

var testFonts = []string{"Fira Code", "Iosevka", "JetBrains Mono", "Menlo"}

func fontOptions(t testing.TB, families []string) Options {
	options := freshOptions(t)
	options.StatePath = options.KnownHostsPath[:strings.LastIndex(options.KnownHostsPath, "known_hosts")] + "state.json"
	options.Keystore = &memKeys{}
	options.MonoFamilies = func() []string { return families }
	return options
}

// waitFor drives the window's wake-ups until condition holds: the font list is
// read off the window thread and arrives through them.
func waitFor(t testing.TB, state *State, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the window")
		}
		state.Wake()
		time.Sleep(2 * time.Millisecond)
	}
}

func TestChoosingAnInstalledFontSwitchesTheTerminalAndIsKept(t *testing.T) {
	options := fontOptions(t, testFonts)
	state := newTestState(t, options)
	state.Click(hitSettings)
	state.selectSettingsCategory(settingsTerminal)
	state.Click(hitSettingsFontPick)
	if state.modal != modalFontPicker || !state.fonts.loading {
		t.Fatalf("modal %d, loading %v", state.modal, state.fonts.loading)
	}
	var canvas ui.Canvas
	state.Paint(&canvas, ui.Size{Width: 1400, Height: 900}) // "Reading the installed fonts…"
	waitFor(t, state, func() bool { return !state.fonts.loading })
	canvas = ui.Canvas{}
	state.Paint(&canvas, ui.Size{Width: 1400, Height: 900})
	texts := canvas.Texts()
	var labels []string
	for _, text := range texts {
		labels = append(labels, text.Value)
	}
	for _, want := range append([]string{state.messages.SettingsTerminalFontSystem, state.messages.SettingsTerminalFontClassic}, testFonts...) {
		if !contains(labels, want) {
			t.Fatalf("the picker does not list %q: %v", want, labels)
		}
	}

	// Typing narrows the list; Enter takes the highlighted row.
	typeInto(state, hitFontPickerSearch, "jet")
	state.Paint(&canvas, ui.Size{Width: 1400, Height: 900})
	if len(state.fonts.filtered) != 1 {
		t.Fatalf("filtered to %d rows", len(state.fonts.filtered))
	}
	before := state.cellWidth
	state.Key(ui.KeyEvent{Key: ui.KeyEnter, Target: hitFontPickerSearch})
	if state.modal != modalNone || state.fonts.family != "JetBrains Mono" || ui.MonoFamilyName(state.terminalFontFace) != "JetBrains Mono" {
		t.Fatalf("modal %d, family %q, face %d", state.modal, state.fonts.family, state.terminalFontFace)
	}
	_ = before
	if !strings.Contains(state.fontDescription(), "JetBrains Mono") {
		t.Fatalf("description %q", state.fontDescription())
	}

	// A new window starts with it, and does not need the list to draw.
	restored := newTestState(t, options)
	if restored.fonts.family != "JetBrains Mono" || ui.MonoFamilyName(restored.terminalFontFace) != "JetBrains Mono" {
		t.Fatalf("restored family %q face %d", restored.fonts.family, restored.terminalFontFace)
	}
}

func TestAFontThatIsNoLongerInstalledFallsBackToTheSystemOne(t *testing.T) {
	options := fontOptions(t, testFonts)
	first := newTestState(t, options)
	first.useTerminalFont(fontEntry{family: "Iosevka"})
	first.save()

	options.MonoFamilies = func() []string { return []string{"Menlo"} } // Iosevka was uninstalled
	second := newTestState(t, options)
	if second.fonts.family != "Iosevka" {
		t.Fatalf("restored family %q", second.fonts.family)
	}
	waitFor(t, second, func() bool { return second.fonts.family == "" })
	if second.terminalFontFace != ui.MonoSystem || !strings.Contains(second.status, "Iosevka") {
		t.Fatalf("face %d, status %q", second.terminalFontFace, second.status)
	}
	if third := newTestState(t, options); third.fonts.family != "" {
		t.Fatalf("the fallback was not saved: %q", third.fonts.family)
	}

	// Still installed: nothing changes.
	options.MonoFamilies = func() []string { return testFonts }
	fourth := newTestState(t, fontOptionsFrom(options, "Fira Code"))
	waitFor(t, fourth, func() bool { return fourth.fonts.installed != nil })
	if fourth.fonts.family != "Fira Code" {
		t.Fatalf("a font that is installed was dropped: %q", fourth.fonts.family)
	}
}

func fontOptionsFrom(options Options, family string) Options {
	state := New(options)
	state.useTerminalFont(fontEntry{family: family})
	state.save()
	state.Close()
	return options
}

func TestFontPickerKeysMoveScrollAndLeaveTheFontAloneOnEscape(t *testing.T) {
	many := make([]string, 40)
	for index := range many {
		many[index] = "Family " + string(rune('A'+index/10)) + string(rune('0'+index%10))
	}
	state := newTestState(t, fontOptions(t, many))
	state.openFontPicker()
	waitFor(t, state, func() bool { return !state.fonts.loading })
	state.refreshFontPicker()
	if len(state.fonts.filtered) != 42 || state.fonts.selected != 0 {
		t.Fatalf("rows %d, selected %d", len(state.fonts.filtered), state.fonts.selected)
	}
	for range 12 {
		state.Key(ui.KeyEvent{Key: ui.KeyDown, Target: hitFontPickerSearch})
	}
	if state.fonts.selected != 12 || state.fonts.top != 12-fontPickerVisibleRows+1 {
		t.Fatalf("selected %d top %d: the window must follow the selection", state.fonts.selected, state.fonts.top)
	}
	state.Key(ui.KeyEvent{Key: ui.KeyPageUp, Target: hitFontPickerSearch})
	state.Key(ui.KeyEvent{Key: ui.KeyPageUp, Target: hitFontPickerSearch})
	state.Key(ui.KeyEvent{Key: ui.KeyUp, Target: hitFontPickerSearch}) // already at the top
	if state.fonts.selected != 0 {
		t.Fatalf("selected %d", state.fonts.selected)
	}
	state.Scroll(ui.ScrollEvent{DeltaY: 100})
	if state.fonts.top != 1 {
		t.Fatalf("wheel moved the list to %d", state.fonts.top)
	}
	before := state.terminalFontFace
	state.Key(ui.KeyEvent{Key: ui.KeyEscape, Target: hitFontPickerSearch})
	if state.modal != modalNone || state.terminalFontFace != before || state.fonts.family != "" {
		t.Fatal("Escape changed the font")
	}

	// Clicking a row chooses it.
	state.openFontPicker()
	state.Click(hitFontPickerItemList.At(3)) // System, Classic, Family A0, Family A1
	if state.fonts.family != "Family A1" || state.modal != modalNone {
		t.Fatalf("family %q modal %d", state.fonts.family, state.modal)
	}
}

func TestTheBuiltInFacesAreChosenFromTheSameList(t *testing.T) {
	state := newTestState(t, fontOptions(t, testFonts))
	state.useTerminalFont(fontEntry{family: "Menlo"})
	state.openFontPicker()
	waitFor(t, state, func() bool { return !state.fonts.loading })
	state.Click(hitFontPickerItemList.At(1)) // Classic
	if state.terminalFontFace != ui.MonoClassic || state.fonts.family != "" {
		t.Fatalf("face %d family %q", state.terminalFontFace, state.fonts.family)
	}
	state.save()
	if saved, _, err := loadState(state.options.StatePath); err != nil || saved.TerminalFontFace != "classic" || saved.TerminalFontFamily != "" {
		t.Fatalf("saved %q %q %v", saved.TerminalFontFace, saved.TerminalFontFamily, err)
	}
}

func TestADamagedFamilyInTheStateFileIsIgnored(t *testing.T) {
	options := fontOptions(t, testFonts)
	if err := saveState(options.StatePath, savedState{TerminalFontFamily: "bad\nname", TerminalFontFace: "classic"}); err != nil {
		t.Fatal(err)
	}
	state := newTestState(t, options)
	if state.fonts.family != "" || state.terminalFontFace != ui.MonoClassic {
		t.Fatalf("family %q face %d", state.fonts.family, state.terminalFontFace)
	}
}
