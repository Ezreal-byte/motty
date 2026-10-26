package workbench

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
	"motty/internal/i18n"
	"motty/internal/update"
)

// Where the project and its community are. A moved group or site is changed in
// one place, so the page and its help text cannot drift apart.
const (
	aboutQQURL      = "https://qm.qq.com/cgi-bin/qm/qr?k=&group_code=1131573299"
	aboutWeChatURL  = "https://docs.qq.com/doc/DVXBGYnFFdm9UWkVP"
	aboutDiscordURL = "https://discord.gg/6s5tn9H4p2"
	aboutIssuesURL  = "https://github.com/t8y2/motty/issues"
	aboutRepoURL    = "https://github.com/t8y2/motty"
	aboutDocsURL    = "https://motty.page"
)

var (
	hitAboutCopyInfo = tree.ID(settingsHits, "settings-about-copy")
	hitAboutQQ       = tree.ID(settingsHits, "settings-about-qq")
	hitAboutWeChat   = tree.ID(settingsHits, "settings-about-wechat")
	hitAboutDiscord  = tree.ID(settingsHits, "settings-about-discord")
	hitAboutIssues   = tree.ID(settingsHits, "settings-about-issues")
	hitAboutRepo     = tree.ID(settingsHits, "settings-about-repo")
	hitAboutDocs     = tree.ID(settingsHits, "settings-about-docs")
)

// aboutBuildInfo is what this copy of the program is: its version, the platform
// it was built for, the Go runtime under it and the commit it came from.
type aboutBuildInfo struct {
	version     string
	versionText string // the version as the page shows it
	platform    string
	build       string // the commit and its date, or "" without one
	runtime     string
	copyText    string // the same facts as one block for a bug report
}

// aboutFacts reads the build once. The strings are made here, not on a paint,
// because a settings page repaints on every pointer move and must not allocate.
var aboutFacts = sync.OnceValue(func() aboutBuildInfo {
	facts := aboutBuildInfo{
		version:  update.Current,
		platform: aboutPlatform(runtime.GOOS, runtime.GOARCH),
		runtime:  runtime.Version(),
	}
	facts.versionText = "v" + facts.version
	if info, ok := debug.ReadBuildInfo(); ok {
		var revision, when string
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				revision = setting.Value
			case "vcs.time":
				when = setting.Value
			}
		}
		if len(revision) > 7 {
			revision = revision[:7]
		}
		if revision != "" {
			facts.build = revision
			if stamp, err := time.Parse(time.RFC3339, when); err == nil {
				facts.build += " · " + stamp.Format("2006-01-02")
			}
		}
	}
	facts.copyText = "Motty " + facts.version + "\nplatform: " + facts.platform + "\nruntime: " + facts.runtime + "\n"
	if facts.build != "" {
		facts.copyText += "build: " + facts.build + "\n"
	}
	return facts
})

// aboutPlatform names the operating system and architecture as a person reads
// them, while keeping the runtime's own names for anything unrecognised.
func aboutPlatform(goos, goarch string) string {
	system := goos
	switch goos {
	case "darwin":
		system = "macOS"
	case "windows":
		system = "Windows"
	case "linux":
		system = "Linux"
	case "freebsd":
		system = "FreeBSD"
	}
	switch goarch {
	case "amd64":
		goarch = "x86-64"
	case "386":
		goarch = "x86"
	}
	return system + " · " + goarch
}

// buildText names the build, with a word of its own when no commit was recorded.
func (facts aboutBuildInfo) buildText(m i18n.AboutMessages) string {
	if facts.build == "" {
		return m.BuildDevelopment
	}
	return facts.build
}

// paintAboutSettings paints the About category: what this copy is, where to
// report a problem and where the project lives.
func (state *State) paintAboutSettings(page *settingsPage) {
	m, pal := state.messages, state.pal
	facts := aboutFacts()
	page.groups(func() {
		page.group("", func() {
			// The wordmark is a picture, so it keeps a slot the page paints
			// once it is laid out, as the sidebar header does.
			row := page.f.Row().Height(96).PadX(18).Gap(16).Items(flex.Center)
			row.Children(func() {
				page.f.Column().Grow(1).Children(func() {
					page.add(settingsControl{kind: controlLogo, slot: page.f.Slot().Size(lockupWidth, lockupHeight).Shrink(0)})
					page.f.Paragraph(m.About.Tagline, 2).FontSize(metrics.FontSizeS).Color(pal.muted).LineHeight(16).MarginSides(6, 0, 0, 0)
				})
				page.f.Text(facts.versionText).Mono().FontSize(metrics.FontSizeS).Color(pal.muted).LineHeight(18).Shrink(0)
			})
		})
		page.group(m.About.InfoTitle, func() {
			page.line(m.About.Platform, facts.platform, 0)
			page.line(m.About.Build, facts.buildText(m.About), 0)
			page.line(m.About.Runtime, facts.runtime, 0)
			copyAction := m.About.CopyAction
			if state.settings.aboutCopied {
				copyAction = m.About.CopiedAction
			}
			page.actionRow(m.About.CopyRow, m.About.CopyDescription, false,
				components.ButtonAction{ID: hitAboutCopyInfo, Label: copyAction, Variant: components.ButtonOutline})
		})
		page.group(m.About.CommunityTitle, func() {
			page.actionRow(m.About.QQ, m.About.QQDescription, false,
				components.ButtonAction{ID: hitAboutQQ, Label: m.About.Join, Variant: components.ButtonOutline})
			page.actionRow(m.About.WeChat, m.About.WeChatDescription, false,
				components.ButtonAction{ID: hitAboutWeChat, Label: m.About.Open, Variant: components.ButtonOutline})
			page.actionRow(m.About.Discord, m.About.DiscordDescription, false,
				components.ButtonAction{ID: hitAboutDiscord, Label: m.About.Join, Variant: components.ButtonOutline})
			page.actionRow(m.About.Issues, m.About.IssuesDescription, false,
				components.ButtonAction{ID: hitAboutIssues, Label: m.About.Open, Variant: components.ButtonOutline})
		})
		page.group(m.About.ProjectTitle, func() {
			page.actionRow(m.About.Repo, m.About.RepoDescription, false,
				components.ButtonAction{ID: hitAboutRepo, Label: m.About.Open, Variant: components.ButtonOutline})
			page.actionRow(m.About.Docs, m.About.DocsDescription, false,
				components.ButtonAction{ID: hitAboutDocs, Label: m.About.Open, Variant: components.ButtonOutline})
		})
	})
}

// clickAboutSettings handles the presses of the About page. It reports whether
// the identity was one of them.
func (state *State) clickAboutSettings(id ui.ID) bool {
	switch id {
	case hitAboutCopyInfo:
		state.copyAboutInfo()
	case hitAboutQQ:
		state.openExternal(aboutQQURL)
	case hitAboutWeChat:
		state.openExternal(aboutWeChatURL)
	case hitAboutDiscord:
		state.openExternal(aboutDiscordURL)
	case hitAboutIssues:
		state.openExternal(aboutIssuesURL)
	case hitAboutRepo:
		state.openExternal(aboutRepoURL)
	case hitAboutDocs:
		state.openExternal(aboutDocsURL)
	default:
		return false
	}
	return true
}

// copyAboutInfo puts the build's facts on the clipboard, ready to paste into a
// bug report.
func (state *State) copyAboutInfo() {
	if err := state.options.SetClipboardText(aboutFacts().copyText); err != nil {
		state.settings.aboutCopied = false
		state.status = fmt.Sprintf(state.messages.StatusClipboardFailedFmt, err)
		return
	}
	state.settings.aboutCopied = true
	state.status = state.messages.About.Copied
}
