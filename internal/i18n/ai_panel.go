package i18n

// AIPanelMessages are the texts of the assistant panel.
type AIPanelMessages struct {
	Title string
	// ToggleTip names the window-bar button; Toggle is the shortcut and command
	// palette entry.
	ToggleTip string
	Toggle    string
	Close     string

	NewConversation string
	History         string
	HistoryTitle    string
	HistoryEmpty    string
	HistoryOpen     string
	HistoryDelete   string
	HistoryRename   string

	// The rename dialog of a saved conversation.
	RenameTitle       string
	RenameField       string
	RenamePlaceholder string
	RenameSubmit      string
	RenameRequired    string

	UsageFmt      string
	UsageTotalFmt string

	ModeAsk   string
	ModeAgent string
	// What the mode switch in the message box does when clicked, for each state.
	ModeHintAsk   string
	ModeHintAgent string

	SetupModel   string
	SetupSummary string

	ModelChooseFmt string // the model chip: the model and what pressing it does
	ResizePanel    string // the name of the edge that changes the panel's width
	ResizeComposer string // and of the one that changes the message box's height

	InputLabel          string
	InputPlaceholder    string
	InputPlaceholderAsk string
	Send                string
	Stop                string

	EmptyTitle  string
	EmptyHint   string
	Example1    string
	Example2    string
	Example3    string
	ScrollToEnd string

	// The empty panel while no terminal is open: what to do to give the assistant
	// something to look at, and questions that need no terminal.
	IdleTitle    string
	IdleHint     string
	IdleExample1 string
	IdleExample2 string
	IdleExample3 string

	Thinking    string
	ThinkingNow string // the same while the model is still at it
	ShowOutput  string
	HideOutput  string
	CopyCode    string
	CopyAnswer  string

	ToolListSessions     string
	ToolTerminalRead     string
	ToolTerminalExec     string
	ToolTerminalExecMany string
	ToolSFTPList         string
	ToolSFTPRead         string
	ToolSFTPWrite        string
	ToolSFTPRemove       string

	StatusRunning string
	StatusWaiting string
	StatusDone    string
	StatusFailed  string
	StatusDenied  string

	RiskLow      string
	RiskMedium   string
	RiskHigh     string
	RiskCritical string

	ApprovalTitle       string
	ApprovalMoreFmt     string // takes how many more are waiting
	ConfirmFmt          string
	ConfirmField        string
	Approve             string
	Decline             string
	ApprovalSafer       string
	EscalationTitle     string
	EscalationSummary   string
	EscalationApprove   string
	EscalationDecline   string
	EscalationCancelled string
	EscalationMissing   string
	NoticeStoreError    string

	MenuAskSelection  string
	MenuExplainOutput string
	PromptSelectionFn string // introduces a quoted selection
	PromptExplain     string
}

var englishAIPanel = AIPanelMessages{
	Title:     "AI assistant",
	ToggleTip: "AI assistant",
	Toggle:    "Toggle AI assistant",
	Close:     "Close assistant",

	NewConversation: "New conversation",
	History:         "Conversation history",
	HistoryTitle:    "History",
	HistoryEmpty:    "No saved conversations yet.",
	HistoryOpen:     "Open conversation",
	HistoryDelete:   "Delete conversation",
	HistoryRename:   "Rename conversation",

	RenameTitle:       "Rename conversation",
	RenameField:       "Title",
	RenamePlaceholder: "Conversation title",
	RenameSubmit:      "Rename",
	RenameRequired:    "A title is required.",

	UsageFmt:      "Tokens: %s in, %s out",
	UsageTotalFmt: " · total %s in, %s out",

	ModeAsk:       "Ask",
	ModeAgent:     "Agent",
	ModeHintAsk:   "Ask: answers only. Click for Agent mode, which can run commands with your approval",
	ModeHintAgent: "Agent: can run commands with your approval. Click to go back to Ask",

	SetupModel:   "Set up a model",
	SetupSummary: "Add a provider and choose a model to start.",

	ModelChooseFmt: "Model: %s. Choose another",
	ResizePanel:    "Resize the assistant panel",
	ResizeComposer: "Resize the message box",

	InputLabel:          "Message to the assistant",
	InputPlaceholder:    "Ask anything (Enter to send)",
	InputPlaceholderAsk: "Ask mode answers only; switch to Agent to run commands.",
	Send:                "Send",
	Stop:                "Stop",

	EmptyTitle:  "Ask about this terminal",
	EmptyHint:   "The assistant sees what is on your screen. In Agent mode it can also run commands for you.",
	Example1:    "Explain the last error and how to fix it",
	Example2:    "What is using the most disk space here?",
	Example3:    "Show me what is listening on which port",
	ScrollToEnd: "Scroll to the latest message",

	IdleTitle:    "Ask anything about SSH and the shell",
	IdleHint:     "Open a host and the assistant can see its terminal; in Agent mode it can also run commands there. Until then it answers from what you ask.",
	IdleExample1: "How do I set up key-based login for a server?",
	IdleExample2: "How do I forward a port to a remote database?",
	IdleExample3: "What is the difference between SFTP and scp?",

	Thinking:    "Thinking",
	ThinkingNow: "Thinking…",
	ShowOutput:  "Show output",
	HideOutput:  "Hide output",
	CopyCode:    "Copy code",
	CopyAnswer:  "Copy answer",

	ToolListSessions:     "List sessions",
	ToolTerminalRead:     "Read terminal",
	ToolTerminalExec:     "Run command",
	ToolTerminalExecMany: "Run command on several hosts",
	ToolSFTPList:         "List directory",
	ToolSFTPRead:         "Read file",
	ToolSFTPWrite:        "Write file",
	ToolSFTPRemove:       "Delete path",

	StatusRunning: "Running",
	StatusWaiting: "Waiting for approval",
	StatusDone:    "Done",
	StatusFailed:  "Failed",
	StatusDenied:  "Declined",

	RiskLow:      "Low risk",
	RiskMedium:   "Medium risk",
	RiskHigh:     "High risk",
	RiskCritical: "Critical risk",

	ApprovalTitle:       "Approval needed",
	ApprovalMoreFmt:     "%d more waiting",
	ConfirmFmt:          "Type %s to confirm",
	ConfirmField:        "Confirmation word",
	Approve:             "Approve",
	Decline:             "Decline",
	ApprovalSafer:       "Safer alternative",
	EscalationTitle:     "Switch to Agent mode and run this?",
	EscalationSummary:   "Switch to Agent mode, then retry the request.",
	EscalationApprove:   "Switch",
	EscalationDecline:   "Stay in Ask",
	EscalationCancelled: "Stayed in Ask mode.",
	EscalationMissing:   "Could not retry: the request was not found.",
	NoticeStoreError:    "The assistant's database could not be opened: %s",

	MenuAskSelection:  "Ask the assistant about the selection",
	MenuExplainOutput: "Explain recent output",
	PromptSelectionFn: "About this part of the terminal:",
	PromptExplain:     "Explain the recent output of this terminal and point out anything wrong.",
}

var chineseAIPanel = AIPanelMessages{
	Title:     "AI 助手",
	ToggleTip: "AI 助手",
	Toggle:    "切换 AI 助手",
	Close:     "关闭助手",

	NewConversation: "新对话",
	History:         "对话历史",
	HistoryTitle:    "历史对话",
	HistoryEmpty:    "还没有保存的对话。",
	HistoryOpen:     "打开对话",
	HistoryDelete:   "删除对话",
	HistoryRename:   "重命名对话",

	RenameTitle:       "重命名对话",
	RenameField:       "标题",
	RenamePlaceholder: "对话标题",
	RenameSubmit:      "重命名",
	RenameRequired:    "标题不能为空。",

	UsageFmt:      "Token:%s 入,%s 出",
	UsageTotalFmt: " · 累计 %s 入,%s 出",

	ModeAsk:       "问答",
	ModeAgent:     "代理",
	ModeHintAsk:   "问答:只回答问题。点击切换到代理模式,经你批准后可以执行命令",
	ModeHintAgent: "代理:经你批准后可以执行命令。点击切回问答",

	SetupModel:   "设置模型",
	SetupSummary: "先添加服务商并选择一个模型。",

	ModelChooseFmt: "模型:%s,点击切换",
	ResizePanel:    "调整助手面板宽度",
	ResizeComposer: "调整输入框高度",

	InputLabel:          "发给助手的消息",
	InputPlaceholder:    "随便问点什么(回车发送)",
	InputPlaceholderAsk: "问答模式只回答问题；要执行命令请切换到代理模式。",
	Send:                "发送",
	Stop:                "停止",

	EmptyTitle:  "就这个终端提问",
	EmptyHint:   "助手能看到屏幕上的内容;在代理模式下,它还可以替你执行命令。",
	Example1:    "解释最近的报错并告诉我怎么修复",
	Example2:    "这里什么占用了最多磁盘空间?",
	Example3:    "看看哪些进程在监听哪些端口",
	ScrollToEnd: "滚动到最新消息",

	IdleTitle:    "问问 SSH 和命令行的事",
	IdleHint:     "先连接一台主机,助手就能看到它的终端;在代理模式下还能替你在上面执行命令。在那之前,它只根据你的提问来回答。",
	IdleExample1: "怎么给服务器配置密钥登录?",
	IdleExample2: "怎么通过端口转发访问远程数据库?",
	IdleExample3: "SFTP 和 scp 有什么区别?",

	Thinking:    "思考",
	ThinkingNow: "思考中…",
	ShowOutput:  "显示输出",
	HideOutput:  "隐藏输出",
	CopyCode:    "复制代码",
	CopyAnswer:  "复制回答",

	ToolListSessions:     "列出会话",
	ToolTerminalRead:     "读取终端",
	ToolTerminalExec:     "执行命令",
	ToolTerminalExecMany: "在多台主机上执行命令",
	ToolSFTPList:         "列出目录",
	ToolSFTPRead:         "读取文件",
	ToolSFTPWrite:        "写入文件",
	ToolSFTPRemove:       "删除路径",

	StatusRunning: "运行中",
	StatusWaiting: "等待批准",
	StatusDone:    "已完成",
	StatusFailed:  "失败",
	StatusDenied:  "已拒绝",

	RiskLow:      "低风险",
	RiskMedium:   "中风险",
	RiskHigh:     "高风险",
	RiskCritical: "极高风险",

	ApprovalTitle:       "需要你的批准",
	ApprovalMoreFmt:     "还有 %d 项在等待",
	ConfirmFmt:          "输入 %s 以确认",
	ConfirmField:        "确认词",
	Approve:             "批准",
	Decline:             "拒绝",
	ApprovalSafer:       "更安全的做法",
	EscalationTitle:     "切换到代理模式并执行？",
	EscalationSummary:   "切换到代理模式后，重新执行这次请求。",
	EscalationApprove:   "切换并执行",
	EscalationDecline:   "继续问答",
	EscalationCancelled: "已保持问答模式。",
	EscalationMissing:   "无法重试：没有找到这条请求。",
	NoticeStoreError:    "无法打开助手的数据库:%s",

	MenuAskSelection:  "就所选内容询问助手",
	MenuExplainOutput: "解释最近的输出",
	PromptSelectionFn: "关于终端中的这一段:",
	PromptExplain:     "请解释这个终端最近的输出,并指出其中的问题。",
}
