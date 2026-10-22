package i18n

// HistoryMessages are the texts of the command history: the picker, its
// Settings rows and the per-host switch.
type HistoryMessages struct {
	Title       string
	Placeholder string
	ScopeHost   string
	ScopeAll    string
	UsedFmt     string
	Hint        string
	Empty       string
	EmptyNone   string
	EmptyOff    string

	SuggestRow          string
	SuggestDescription  string
	SettingsRow         string
	SettingsDescription string
	ClearRow            string
	ClearDescription    string
	Clear               string
	ClearTitle          string
	ClearConfirm        string
	HostNoRecord        string

	StatusCleared         string
	StatusClearFailedFmt  string
	StatusUnavailableFmt  string
	StatusNoTerminal      string
	StatusNotConnected    string
	StatusRemovedFmt      string
	StatusRemoveFailedFmt string
}

var englishHistory = HistoryMessages{
	Title:       "Command history",
	Placeholder: "Search the commands you typed",
	ScopeHost:   "This host",
	ScopeAll:    "All hosts",
	UsedFmt:     "×%d",
	Hint:        "Enter inserts · Shift+Enter runs · Delete removes",
	Empty:       "No matching command.",
	EmptyNone:   "No commands remembered yet.",
	EmptyOff:    "Command history is off. Turn on Remember commands in Settings > Terminal.",

	SuggestRow:          "Inline suggestions",
	SuggestDescription:  "Right arrow or End takes the rest, Alt or Ctrl+Right a word. Needs Remember commands.",
	SettingsRow:         "Remember commands",
	SettingsDescription: "Keeps only lines the screen echoed, so passwords are never saved; stays out of backups.",
	ClearRow:            "Command history",
	ClearDescription:    "Forget every remembered command on every host.",
	Clear:               "Clear",
	ClearTitle:          "Clear the command history?",
	ClearConfirm:        "Clear history",
	HostNoRecord:        "Don't record commands",

	StatusCleared:         "Command history cleared.",
	StatusClearFailedFmt:  "Could not clear the command history: %s",
	StatusUnavailableFmt:  "Command history is unavailable: %s",
	StatusNoTerminal:      "Open a terminal first.",
	StatusNotConnected:    "The terminal is not connected.",
	StatusRemovedFmt:      "Removed from the history: %s",
	StatusRemoveFailedFmt: "Could not remove it from the history: %s",
}

var chineseHistory = HistoryMessages{
	Title:       "命令历史",
	Placeholder: "搜索输入过的命令",
	ScopeHost:   "当前主机",
	ScopeAll:    "所有主机",
	UsedFmt:     "%d 次",
	Hint:        "回车插入 · Shift+回车执行 · Delete 删除",
	Empty:       "没有匹配的命令。",
	EmptyNone:   "还没有记住任何命令。",
	EmptyOff:    "命令历史已关闭。请在 设置 > 终端 中打开“记住命令”。",

	SuggestRow:          "行内建议",
	SuggestDescription:  "按 → 或 End 接受整条，Alt 或 Ctrl+→ 接受一个词；需先开启“记住命令”。",
	SettingsRow:         "记住命令",
	SettingsDescription: "只记录屏幕回显过的命令，密码不会被保存；仅存本机，不进备份。",
	ClearRow:            "命令历史",
	ClearDescription:    "忘掉所有主机上记住的全部命令。",
	Clear:               "清除",
	ClearTitle:          "清除命令历史?",
	ClearConfirm:        "清除历史",
	HostNoRecord:        "不记录命令",

	StatusCleared:         "命令历史已清除。",
	StatusClearFailedFmt:  "无法清除命令历史:%s",
	StatusUnavailableFmt:  "命令历史不可用:%s",
	StatusNoTerminal:      "请先打开一个终端。",
	StatusNotConnected:    "终端尚未连接。",
	StatusRemovedFmt:      "已从历史中移除:%s",
	StatusRemoveFailedFmt: "无法从历史中移除:%s",
}
