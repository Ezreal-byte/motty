package workbench

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
	"motty/internal/update"
)

// What the system and the command line can ask of a running window: connect
// somewhere. A target written on the command line is what the user typed, so it
// connects at once; a link (ssh://host) comes from another program, a web page
// perhaps, so the window asks before connecting.

// launchHits is the place of the controls this file paints.
var launchHits = tree.Feature("launch")

var (
	hitLaunchDismiss = tree.ID(launchHits, "launch-dismiss")
	hitLaunchClose   = tree.ID(launchHits, "launch-close")
	hitLaunchCancel  = tree.ID(launchHits, "launch-cancel")
	hitLaunchConnect = tree.ID(launchHits, "launch-connect")

	hitUpdateDismiss  = tree.ID(launchHits, "update-dismiss")
	hitUpdateClose    = tree.ID(launchHits, "update-close")
	hitUpdateSkip     = tree.ID(launchHits, "update-skip")
	hitUpdateLater    = tree.ID(launchHits, "update-later")
	hitUpdateDownload = tree.ID(launchHits, "update-download")
)

var (
	hitSettingsLoginToggle       = tree.ID(launchHits, "settings-login")
	hitSettingsUpdateCheck       = tree.ID(launchHits, "settings-update-check")
	hitSettingsUpdateDownload    = tree.ID(launchHits, "settings-update-download")
	hitSettingsUpdateStartToggle = tree.ID(launchHits, "settings-update-start")
	hitSettingsDiagnostics       = tree.ID(launchHits, "settings-diagnostics")
)

// LoginStart reads and changes whether the program starts when the user signs in.
type LoginStart struct {
	Enabled func() (bool, error)
	Set     func(bool) error
}

// launchText turns a command line into the text a connection is parsed from, and
// says whether it is a link. It understands what ssh and PuTTY users type:
//
//	user@host            ssh://user@host:2222/       telnet://host
//	ssh -p 2222 host     -ssh -P 2222 -l user host
func launchText(args []string) (text string, link bool) {
	words := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "":
			continue
		case "-ssh":
			continue // PuTTY's protocol switch; ssh is the default
		case "-P":
			arg = "-p"
		case "--background":
			continue // what the login entry adds; it is not a target
		}
		if strings.HasPrefix(arg, "-P") && len(arg) > 2 && arg[2] >= '0' && arg[2] <= '9' {
			arg = "-p" + arg[2:]
		}
		words = append(words, arg)
	}
	text = strings.Join(words, " ")
	lower := strings.ToLower(text)
	link = strings.HasPrefix(lower, "ssh://") || strings.HasPrefix(lower, "telnet://")
	return text, link
}

// Launch handles a request from outside to connect, with the arguments of a
// command line or one link. It is safe from any goroutine: the work happens on
// the window thread, which is woken for it.
func (state *State) Launch(args []string) {
	args = append([]string(nil), args...)
	state.aiPost(func(state *State) {
		state.launchBacklog = append(state.launchBacklog, args)
		state.runLaunches()
	})
}

// afterEvent starts requests that were waiting for a dialog the event closed. A
// closed dialog does not wake the window, so without this they would sit until
// something else did.
func (state *State) afterEvent(result *ui.EventResult) {
	if state.runLaunches() {
		result.Repaint = true
	}
}

// runLaunches starts the requests that came in, one at a time, while no dialog
// is in the way. It reports whether it started any.
func (state *State) runLaunches() bool {
	ran := len(state.launchBacklog) > 0 && state.launchReady()
	for len(state.launchBacklog) > 0 && state.launchReady() {
		args := state.launchBacklog[0]
		state.launchBacklog = slices.Delete(state.launchBacklog, 0, 1)
		state.openLaunch(args)
	}
	return ran
}

func (state *State) launchReady() bool {
	return state.modal == modalNone && state.knownHosts.action == knownHostsActionNone
}

func (state *State) openLaunch(args []string) {
	text, link := launchText(args)
	if text == "" {
		return
	}
	if state.options.Raise != nil {
		state.options.Raise()
	}
	parsed, err := parseConnection(text)
	if err != nil {
		state.activeSection = sectionHosts
		state.status = fmt.Sprintf(state.messages.ErrBadTargetFmt, text)
		return
	}
	if !link {
		state.connectParsed(parsed)
		return
	}
	state.launchTarget = parsed
	state.modal = modalOpenLink
}

// launchDescription names what the link asks to open.
func (state *State) launchDescription() string {
	target := state.launchTarget
	where := net.JoinHostPort(target.host, strconv.Itoa(int(target.port)))
	if target.user != "" {
		where = target.user + "@" + where
	}
	return fmt.Sprintf(state.messages.Tools.LaunchDescriptionFmt, protocolName(target.protocol), where)
}

func (state *State) clickLaunchDialog(id ui.ID) {
	switch id {
	case hitLaunchDismiss, hitLaunchClose, hitLaunchCancel:
		state.modal = modalNone
	case hitLaunchConnect:
		state.modal = modalNone
		state.connectParsed(state.launchTarget)
	}
}

func (state *State) launchKey(event ui.KeyEvent) (ui.EventResult, bool) {
	if state.modal != modalOpenLink || event.Key != ui.KeyEnter || event.Target != hitLaunchConnect || event.Repeat {
		return ui.EventResult{}, false
	}
	state.clickLaunchDialog(hitLaunchConnect)
	return ui.EventResult{Handled: true, Repaint: true}, true
}

func (state *State) paintLaunchDialog(canvas *ui.Canvas, viewport ui.Rect) {
	m := state.messages
	dialog := components.BeginDialog(canvas, state.Theme(), viewport, components.DialogOptions{
		Layer: dialogLayer, Scope: dialogScope,
		DismissID: hitLaunchDismiss, CloseID: hitLaunchClose, InitialFocusID: hitLaunchCancel,
		ContentSize: ui.Size{Width: 520}, FitBody: true, BodyHeight: 3 * 18, // the warning's lines
	})
	layout := dialog.Begin(m.Tools.LaunchTitle, state.launchDescription())
	f := state.settingsRow.Begin(canvas, state.Theme(), layout.Body)
	f.Column().Fill().Children(func() {
		f.Paragraph(m.Tools.LaunchWarning, 3).FontSize(metrics.FontSizeS).Color(state.pal.muted).LineHeight(18)
	})
	f.End()
	dialog.Actions(layout,
		components.ButtonAction{ID: hitLaunchCancel, Label: m.Cancel, Variant: components.ButtonSecondary},
		components.ButtonAction{ID: hitLaunchConnect, Label: m.Tools.LaunchConnect},
	)
	dialog.End()
}

// updateNotesLines is how many lines of release notes the update dialog shows.
const updateNotesLines = 9

func (state *State) paintUpdateDialog(canvas *ui.Canvas, viewport ui.Rect) {
	m := state.messages.Tools
	dialog := components.BeginDialog(canvas, state.Theme(), viewport, components.DialogOptions{
		Layer: dialogLayer, Scope: dialogScope,
		DismissID: hitUpdateDismiss, CloseID: hitUpdateClose, InitialFocusID: hitUpdateDownload,
		ContentSize: ui.Size{Width: 560}, FitBody: true, BodyHeight: updateNotesLines * 20,
	})
	layout := dialog.Begin(fmt.Sprintf(m.UpdateDialogTitleFmt, state.update.release.Version), fmt.Sprintf(m.UpdateDialogDescriptionFmt, update.Current))
	notes := update.PlainNotes(state.update.release.Notes)
	if notes == "" {
		notes = m.UpdateNotesEmpty
	}
	f := state.settingsRow.Begin(canvas, state.Theme(), layout.Body)
	f.Column().Fill().Children(func() {
		f.Paragraph(notes, updateNotesLines).FontSize(metrics.FontSizeS).Color(state.pal.muted).LineHeight(20)
	})
	f.End()
	dialog.Actions(layout,
		components.ButtonAction{ID: hitUpdateSkip, Label: m.UpdateSkip, Variant: components.ButtonOutline},
		components.ButtonAction{ID: hitUpdateLater, Label: m.UpdateLater, Variant: components.ButtonSecondary},
		components.ButtonAction{ID: hitUpdateDownload, Label: m.GeneralUpdateDownload},
	)
	dialog.End()
}

func (state *State) clickUpdateDialog(id ui.ID) {
	switch id {
	case hitUpdateDismiss, hitUpdateClose, hitUpdateLater:
	case hitUpdateSkip:
		state.update.skipped = state.update.release.Version
		state.save()
	case hitUpdateDownload:
		state.openExternal(state.update.release.URL)
	default:
		return
	}
	state.closeModal()
	state.showNextPrompt()
}

func (state *State) updateKey(event ui.KeyEvent) (ui.EventResult, bool) {
	if state.modal != modalUpdate || event.Key != ui.KeyEnter || event.Repeat {
		return ui.EventResult{}, false
	}
	switch event.Target {
	case hitUpdateDownload, hitUpdateSkip, hitUpdateLater:
		state.clickUpdateDialog(event.Target)
		return ui.EventResult{Handled: true, Repaint: true}, true
	}
	return ui.EventResult{}, false
}

// ---- the General settings: starting at login and updates ----

// updateState is what the updates row shows.
type updateState struct {
	checking  bool
	release   update.Release
	available bool
	text      string // the outcome of the last check, in the language it was made in
	offer     bool   // a dialog about release is waiting for its turn
	skipped   string // the version the user asked not to be reminded of
}

func (state *State) settingsLoginEnabled() bool {
	if state.options.StartAtLogin == nil {
		return false
	}
	on, err := state.options.StartAtLogin.Enabled()
	return err == nil && on
}

// paintLoginSetting is the start-at-login row, in the first group of the page.
func (state *State) paintLoginSetting(page *settingsPage) {
	m := state.messages.Tools
	page.toggleRow(m.GeneralLogin, m.GeneralLoginDescription, state.loginOn, hitSettingsLoginToggle)
}

func (state *State) paintUpdateSettings(page *settingsPage) {
	m := state.messages.Tools
	page.group("", func() {
		page.toggleRow(m.GeneralUpdateStartup, m.GeneralUpdateStartupDescription, state.updateAtStart, hitSettingsUpdateStartToggle)
		description := m.GeneralUpdateDescription
		if state.update.text != "" {
			description = state.update.text
		}
		actions := []components.ButtonAction{{ID: hitSettingsUpdateCheck, Label: m.GeneralUpdateCheck, Variant: components.ButtonOutline, Disabled: state.update.checking}}
		if state.update.available {
			actions = append(actions, components.ButtonAction{ID: hitSettingsUpdateDownload, Label: m.GeneralUpdateDownload})
		}
		page.actionRow(m.GeneralUpdate, description, false, actions...)
	})
	page.group("", func() {
		page.actionRow(m.DiagnosticsRow, m.DiagnosticsDescription, false, components.ButtonAction{ID: hitSettingsDiagnostics, Label: m.DiagnosticsExport, Variant: components.ButtonOutline})
	})
}

// clickLaunchSettings handles the presses of those settings.
func (state *State) clickLaunchSettings(id ui.ID) bool {
	switch id {
	case hitSettingsLoginToggle:
		if state.options.StartAtLogin == nil {
			return true
		}
		want := !state.loginOn
		if err := state.options.StartAtLogin.Set(want); err != nil {
			state.status = fmt.Sprintf(state.messages.Tools.LoginFailedFmt, err)
		} else {
			state.loginOn = want
		}
	case hitSettingsUpdateStartToggle:
		state.updateAtStart = !state.updateAtStart
		state.save()
	case hitSettingsUpdateCheck:
		state.checkForUpdates(true)
	case hitSettingsDiagnostics:
		state.openExportDiagnostics()
	case hitSettingsUpdateDownload:
		if state.update.available {
			state.openExternal(state.update.release.URL)
		}
	default:
		return false
	}
	return true
}

// checkForUpdates asks the release feed on a goroutine and shows the answer when
// it comes. A check the user did not ask for stays quiet unless there is news.
func (state *State) checkForUpdates(asked bool) {
	if state.update.checking {
		return
	}
	state.update.checking = true
	if asked {
		state.update.text = state.messages.Tools.UpdateChecking
	}
	check := state.options.CheckUpdate
	if check == nil {
		check = func() (update.Release, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			return update.Latest(ctx, nil, update.Feed)
		}
	}
	go func() {
		release, err := check()
		state.aiPost(func(state *State) {
			state.update.checking = false
			m := state.messages.Tools
			switch {
			case errors.Is(err, update.ErrNoRelease):
				if asked {
					state.update.text = m.UpdateNoReleaseText
				} else {
					state.update.text = ""
				}
			case err != nil:
				if asked {
					state.update.text = fmt.Sprintf(m.UpdateFailedFmt, err)
				} else {
					state.update.text = ""
				}
			case update.Newer(update.Current, release.Version):
				state.update.release, state.update.available = release, true
				state.update.text = fmt.Sprintf(m.UpdateAvailableFmt, release.Version)
				state.status = state.update.text
				// A check the user asked for always answers; one made at startup
				// leaves alone a version that was skipped.
				if asked || release.Version != state.update.skipped {
					state.update.offer = true
					state.showNextPrompt()
				}
			default:
				state.update.available = false
				if asked {
					state.update.text = fmt.Sprintf(m.UpdateLatestFmt, update.Current)
				} else {
					state.update.text = ""
				}
			}
		})
	}()
}

// openExternal opens a web address in the browser.
func (state *State) openExternal(url string) {
	if err := state.options.OpenURL(url); err != nil {
		state.status = fmt.Sprintf(state.messages.StatusOpenURLFailedFmt, err)
	}
}

func protocolName(protocol hostProtocol) string {
	switch protocol {
	case protocolTelnet:
		return "Telnet"
	case protocolSerial:
		return "serial"
	case protocolVNC:
		return "VNC"
	case protocolRDP:
		return "RDP"
	}
	return "SSH"
}

func updateCheckString(on bool) string {
	if on {
		return "on"
	}
	return ""
}
