package workbench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
	"motty/internal/ai/store"
	"motty/internal/i18n"
)

// The settings of the MCP server that works without the window (`motty
// mcp-serve`, see agent_mcp.go): what agents may do, which hosts they have, the
// client configuration that starts it, and what they did.

var (
	hitAISetAgentObserve = tree.ID(aiSettingsHits, "ai-set-agent-observe")
	hitAISetAgentRun     = tree.ID(aiSettingsHits, "ai-set-agent-run")
	hitAISetAgentCopy    = tree.ID(aiSettingsHits, "ai-set-agent-copy")
	hitAISetAgentClear   = tree.ID(aiSettingsHits, "ai-set-agent-clear")
)

// agentAuditShown is how many of the latest calls the page lists.
const agentAuditShown = 5

func (state *State) aiAgentModeOptions() [2]components.SegmentOption {
	m := state.messages.AISettings
	return [...]components.SegmentOption{
		{ID: hitAISetAgentObserve, Label: m.AgentModeObserve},
		{ID: hitAISetAgentRun, Label: m.AgentModeRun},
	}
}

// aiAgentClick handles the controls of the group.
func (state *State) aiAgentClick(id ui.ID) bool {
	switch id {
	case hitAISetAgentObserve:
		state.aiPut(agentModeSetting, AgentModeReadOnly)
	case hitAISetAgentRun:
		state.aiPut(agentModeSetting, AgentModeCommands)
	case hitAISetAgentCopy:
		state.aiCopyText(hitAISetAgentCopy, aiAgentConfig(state.ai.cfg.mcpClient, state.agentExecutable()))
	case hitAISetAgentClear:
		if db, err := state.aiStore(); err == nil && db.ClearAudit() == nil {
			state.aiWrote()
		}
	default:
		return false
	}
	return true
}

// agentExecutable is the program a client has to start.
func (state *State) agentExecutable() string {
	path, err := state.options.Executable()
	if err != nil {
		return "motty"
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return path
}

// agentHostLabels are the names of the hosts open to agents, in catalog order.
func (state *State) agentHostLabels() []string {
	var labels []string
	for _, host := range state.agentHostList() {
		labels = append(labels, host.Label)
	}
	return labels
}

// agentSummary caches the texts the page shows about agents, which are made of
// data that changes rarely, so that painting an unchanged page allocates nothing.
type agentSummary struct {
	hostsHash  uint64
	hostsCount int
	hostsText  string

	lines    []string
	language i18n.Language
}

// agentHostsText lists the hosts open to agents, or says there are none. It is
// rebuilt only when the hosts that are open, or their names, are not the ones it
// was made from.
func (state *State) agentHostsText() string {
	const offset, prime = 14695981039346656037, 1099511628211
	hash, count := uint64(offset), 0
	for index := range state.hosts {
		host := &state.hosts[index]
		if !host.isSSH() || !state.agentHosts[host.CatalogID] {
			continue
		}
		count++
		for i := 0; i < len(host.Label); i++ {
			hash = (hash ^ uint64(host.Label[i])) * prime
		}
		hash = (hash ^ 0xff) * prime
	}
	if count == 0 {
		return state.messages.AISettings.AgentHostsNoneDesc
	}
	cache := &state.ai.agentSummary
	if cache.hostsCount != count || cache.hostsHash != hash || cache.hostsText == "" {
		cache.hostsHash, cache.hostsCount = hash, count
		cache.hostsText = strings.Join(state.agentHostLabels(), ", ")
	}
	return cache.hostsText
}

// agentAuditLines are the latest calls as the page shows them, in the language
// of the window; a reload of the log or a change of language rebuilds them.
func (state *State) agentAuditLines() []string {
	cache := &state.ai.agentSummary
	entries := state.ai.agentAudit
	if len(entries) == 0 {
		return nil
	}
	if cache.lines == nil || cache.language != state.language {
		cache.lines = cache.lines[:0]
		for _, entry := range entries[:min(len(entries), agentAuditShown)] {
			cache.lines = append(cache.lines, agentAuditLine(entry, state.messages.AISettings))
		}
		cache.language = state.language
	}
	return cache.lines
}

func (state *State) paintAIAgents(page *settingsPage) {
	pal, m := state.pal, state.messages.AISettings
	selected := hitAISetAgentObserve
	if state.ai.settings.agentRun {
		selected = hitAISetAgentRun
	}
	modes := state.aiAgentModeOptions()
	hosts := state.agentHostsText()
	entries := state.ai.agentAudit
	activity := m.AgentActivityNone
	if len(entries) > 0 {
		activity = m.AgentActivityDesc
	}
	page.group(m.AgentsTitle, func() {
		page.mcpFrame(m.AgentMode, m.AgentModeDesc, pal.muted, func() {
			page.segmentSlot(min(float32(220), page.width*0.34), modes[:], selected)
		})
		page.mcpFrame(m.AgentHosts, hosts, pal.muted, func() {})
		page.mcpFrame(m.AgentConfig, m.AgentConfigDesc, pal.muted, func() {
			page.mcpCopyButton(hitAISetAgentCopy, m.MCPCopy, false)
		})
		page.mcpFrame(m.AgentActivity, activity, pal.muted, func() {
			page.button(hitAISetAgentClear, m.AgentActivityClear, aiButtonOutline, len(entries) == 0)
		})
		if len(entries) == 0 {
			return
		}
		page.separator()
		f := page.f
		f.Column().Pad(18).Gap(4).Children(func() {
			for _, line := range state.agentAuditLines() {
				f.Wrap(line).FontSize(metrics.FontSizeS).Color(pal.muted).LineHeight(16)
			}
		})
	})
}

// agentAuditLine is one call of the activity list: when, which host, which tool,
// the command or path, and how it ended.
func agentAuditLine(entry store.AuditEntry, m i18n.AISettingsMessages) string {
	outcome := m.AgentOutcomeOK
	switch entry.Outcome {
	case store.AuditFailed:
		outcome = m.AgentOutcomeFailed
	case store.AuditRefused:
		outcome = m.AgentOutcomeRefused
	}
	what := entry.Tool
	if entry.Summary != "" {
		what += " " + entry.Summary
	}
	host := entry.HostLabel
	if host == "" {
		host = "-"
	}
	return fmt.Sprintf("%s  %s  %s — %s", entry.At.Local().Format(time.DateTime), host, what, outcome)
}

// aiCopyText puts text on the clipboard and remembers which button did, so the
// button can say it worked.
func (state *State) aiCopyText(target ui.ID, value string) {
	cfg := &state.ai.cfg
	cfg.copyTarget = target
	if err := state.options.SetClipboardText(value); err != nil {
		cfg.copied = aiCopyFailed
		return
	}
	cfg.copied = aiCopyDone
}

// aiAgentConfig is what a client needs to start `motty mcp-serve` itself: the
// program and its one argument. There is no address and no token. The forms
// follow each client's own way of naming a program to start; clients whose
// stdio form is not documented get the common mcpServers entry.
func aiAgentConfig(client aiMCPClient, executable string) string {
	const name = "motty-hosts"
	args := []string{"mcp-serve"}
	switch client {
	case aiMCPClaude:
		return "claude mcp add " + name + " -- " + shellWord(executable) + " mcp-serve"
	case aiMCPCodeBuddy:
		return "codebuddy mcp add --scope local -- " + name + " " + shellWord(executable) + " mcp-serve"
	case aiMCPCodex:
		// JSON string escaping is also valid for TOML basic strings.
		command, _ := json.Marshal(executable)
		return fmt.Sprintf("[mcp_servers.%s]\ncommand = %s\nargs = [\"mcp-serve\"]", name, command)
	case aiMCPDeepSeek:
		command, _ := json.Marshal(executable)
		return fmt.Sprintf("- insert:\n    - id: mcp-%s\n      name: '@deepseek-ai/dsh-mcp-client'\n      config:\n        serverName: %s\n        transport: stdio\n        command: %s\n        args: [\"mcp-serve\"]", name, name, command)
	}
	server := map[string]any{"command": executable, "args": args}
	root := "mcpServers"
	switch client {
	case aiMCPZCode, aiMCPTrae, aiMCPQoder, aiMCPWorkBuddy:
		server["type"] = "stdio"
	case aiMCPVSCode:
		root = "servers"
		server["type"] = "stdio"
	case aiMCPOpenCode:
		root = "mcp"
		server = map[string]any{"type": "local", "command": append([]string{executable}, args...), "enabled": true}
	case aiMCPCherry:
		server["type"], server["name"] = "stdio", name
	}
	var config any = map[string]any{root: map[string]any{name: server}}
	if client == aiMCPZCode {
		config = map[string]any{"mcp": map[string]any{"servers": map[string]any{name: server}}}
	}
	value, _ := json.MarshalIndent(config, "", "  ")
	return string(value)
}

var plainShellWord = regexp.MustCompile(`^[A-Za-z0-9_./:@%+=,\\-]+$`)

// shellWord quotes a path for the command line of the user's shell, so a path
// with a space survives: double quotes where the shell is Windows', single
// quotes elsewhere.
func shellWord(word string) string {
	if plainShellWord.MatchString(word) {
		return word
	}
	if runtime.GOOS == "windows" {
		return `"` + strings.ReplaceAll(word, `"`, `\"`) + `"`
	}
	return "'" + strings.ReplaceAll(word, "'", `'\''`) + "'"
}

// executableOf is Options.Executable's default.
func executableOf() (string, error) { return os.Executable() }
