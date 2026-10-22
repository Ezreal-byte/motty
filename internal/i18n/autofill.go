package i18n

// AutofillMessages are the texts of the autofill rules: the Settings group, the
// dialog that edits a rule and the confirmation before a rule first answers.
type AutofillMessages struct {
	Group              string
	Row                string
	RowDescription     string
	Add                string
	Edit               string
	Delete             string
	TurnOn             string
	TurnOff            string
	SendPassword       string
	SendOTP            string
	SendText           string
	ScopeAll           string
	ScopeFmt           string
	StateOn            string
	StateOff           string
	WithEnter          string
	DialogAddTitle     string
	DialogEditTitle    string
	Name               string
	NamePlaceholder    string
	Pattern            string
	PatternPlaceholder string
	Send               string
	FixedText          string
	FixedTextHint      string
	Enter              string
	Scope              string
	ScopePlaceholder   string
	Enabled            string
	Save               string

	ErrorName        string
	ErrorPattern     string
	ErrorTooLong     string
	ErrorMatchesAll  string
	ErrorInvalidFmt  string
	ErrorTooMany     string
	ErrorFixedText   string
	ErrorScopeFmt    string
	ErrorVaultLocked string
	ErrorVaultFmt    string

	ConfirmTitle    string
	ConfirmFmt      string
	ConfirmSendsFmt string
	ConfirmLine     string
	ConfirmAllow    string
	StatusFiredFmt  string
	StatusNoSecret  string
	StatusLockedFmt string
	StatusDeclined  string
}

var englishAutofill = AutofillMessages{
	Group:              "Autofill rules",
	Row:                "Autofill rules",
	RowDescription:     "Answer a prompt in the terminal by itself when its last line matches a pattern. A new rule starts off.",
	Add:                "Add rule",
	Edit:               "Edit",
	Delete:             "Delete",
	TurnOn:             "Turn on",
	TurnOff:            "Turn off",
	SendPassword:       "saved password",
	SendOTP:            "one-time code",
	SendText:           "fixed text",
	ScopeAll:           "all hosts",
	ScopeFmt:           "hosts: %s",
	StateOn:            "on",
	StateOff:           "off",
	WithEnter:          "then Enter",
	DialogAddTitle:     "Add autofill rule",
	DialogEditTitle:    "Edit autofill rule",
	Name:               "Name",
	NamePlaceholder:    "sudo password",
	Pattern:            "Prompt pattern (Go regular expression, matched against the last line)",
	PatternPlaceholder: `\[sudo\] password for \w+:`,
	Send:               "Send",
	FixedText:          "Fixed text (kept encrypted in the vault)",
	FixedTextHint:      "Leave empty to keep the stored text",
	Enter:              "Press Enter after",
	Scope:              "Hosts or groups (comma separated; empty for all hosts)",
	ScopePlaceholder:   "all hosts",
	Enabled:            "Turn the rule on",
	Save:               "Save rule",

	ErrorName:        "Give the rule a name.",
	ErrorPattern:     "Write the pattern that the prompt matches.",
	ErrorTooLong:     "The pattern is too long.",
	ErrorMatchesAll:  "The pattern matches an empty line, so it would match everything.",
	ErrorInvalidFmt:  "Not a valid regular expression: %s",
	ErrorTooMany:     "There are too many rules already.",
	ErrorFixedText:   "Write the text to send.",
	ErrorScopeFmt:    "No host or group is named %q.",
	ErrorVaultLocked: "The vault is locked. Unlock it in Settings first.",
	ErrorVaultFmt:    "Could not store the text: %s",

	ConfirmTitle:    "Autofill rule wants to answer",
	ConfirmFmt:      "Rule %q matched a prompt on %s.",
	ConfirmSendsFmt: "It will send the %s. This is asked once per session.",
	ConfirmLine:     "Prompt",
	ConfirmAllow:    "Send",
	StatusFiredFmt:  "Autofill rule %q answered a prompt.",
	StatusNoSecret:  "Autofill rule %q matched, but nothing is saved to send for this host.",
	StatusLockedFmt: "Autofill rule %q matched, but the vault is locked.",
	StatusDeclined:  "Autofill rule did not send.",
}

var chineseAutofill = AutofillMessages{
	Group:              "自动填充规则",
	Row:                "自动填充规则",
	RowDescription:     "终端最后一行匹配正则时自动回答提示。新建的规则默认关闭。",
	Add:                "添加规则",
	Edit:               "编辑",
	Delete:             "删除",
	TurnOn:             "开启",
	TurnOff:            "关闭",
	SendPassword:       "已保存的密码",
	SendOTP:            "一次性验证码",
	SendText:           "固定文本",
	ScopeAll:           "所有主机",
	ScopeFmt:           "主机:%s",
	StateOn:            "已开启",
	StateOff:           "已关闭",
	WithEnter:          "并回车",
	DialogAddTitle:     "添加自动填充规则",
	DialogEditTitle:    "编辑自动填充规则",
	Name:               "名称",
	NamePlaceholder:    "sudo 密码",
	Pattern:            "提示正则（Go 正则表达式，匹配终端最后一行）",
	PatternPlaceholder: `\[sudo\] password for \w+:`,
	Send:               "发送内容",
	FixedText:          "固定文本(加密保存在保险库)",
	FixedTextHint:      "留空则保留已存的文本",
	Enter:              "发送后按回车",
	Scope:              "主机或分组(逗号分隔,留空为所有主机)",
	ScopePlaceholder:   "所有主机",
	Enabled:            "开启此规则",
	Save:               "保存规则",

	ErrorName:        "请给规则起个名字。",
	ErrorPattern:     "请填写提示所匹配的正则。",
	ErrorTooLong:     "正则太长。",
	ErrorMatchesAll:  "该正则能匹配空行,会匹配所有内容。",
	ErrorInvalidFmt:  "不是有效的正则表达式:%s",
	ErrorTooMany:     "规则已经太多了。",
	ErrorFixedText:   "请填写要发送的文本。",
	ErrorScopeFmt:    "没有名为 %q 的主机或分组。",
	ErrorVaultLocked: "保险库已锁定,请先在设置中解锁。",
	ErrorVaultFmt:    "无法保存文本:%s",

	ConfirmTitle:    "自动填充规则要回答提示",
	ConfirmFmt:      "规则 %q 在 %s 上匹配到提示。",
	ConfirmSendsFmt: "将发送%s。每个会话只询问一次。",
	ConfirmLine:     "提示",
	ConfirmAllow:    "发送",
	StatusFiredFmt:  "自动填充规则 %q 已回答提示。",
	StatusNoSecret:  "自动填充规则 %q 已匹配,但这台主机没有可发送的已保存内容。",
	StatusLockedFmt: "自动填充规则 %q 已匹配,但保险库已锁定。",
	StatusDeclined:  "自动填充规则未发送。",
}
