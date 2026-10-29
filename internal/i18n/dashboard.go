package i18n

type DashboardMessages struct {
	View, Metrics, Usage                                                          string
	Host, Mount, Interface, Used, Total, Available, ReceivedTotal, SentTotal      string
	Title, New, Edit, Delete, Start, Stop, Simple, Detailed, Search               string
	Name, Note, Hosts, HostSearch, Create, Save, Empty, EmptyHint                 string
	NameRequired, DuplicateName, HostsRequired, DeletePrompt                      string
	Sampling, Retention, Every, Minutes, Hours, Count, Collecting                 string
	Waiting, Connecting, Live, Stopped, Failed, Unsupported, Missing, Updated     string
	State, CPU, Memory, Disk, Network, Receive, Send, Terminal, Retry, PickerHint string
}

var chineseDashboard = DashboardMessages{
	View: "视图", Metrics: "资源指标", Usage: "使用率",
	Host: "主机", Mount: "挂载路径", Interface: "网络接口", Used: "已用", Total: "总容量", Available: "可用", ReceivedTotal: "累计接收", SentTotal: "累计发送",
	Title: "监控", New: "新建面板", Edit: "编辑当前面板", Delete: "删除当前面板", Start: "开始监控", Stop: "停止监控", Simple: "简洁", Detailed: "详细", Search: "搜索主机、地址或标签",
	Name: "面板名称", Note: "备注（可选）", Hosts: "选择主机", HostSearch: "搜索名称、地址或分组", Create: "创建面板", Save: "保存更改", Empty: "创建你的第一个监控面板", EmptyHint: "选择需要关注的主机，开始监控后查看资源使用情况。",
	NameRequired: "请输入面板名称（最多 40 字）", DuplicateName: "已存在同名面板", HostsRequired: "请至少选择一台 SSH 主机", DeletePrompt: "删除此监控面板？正在进行的采集将停止，面板历史也会清除。",
	Sampling: "采集", Retention: "保留", Every: "每 %d 秒", Minutes: "%d 分钟", Hours: "%d 小时", Count: "共 %d 台主机", Collecting: "%d / %d 主机采集中",
	Waiting: "未开始", Connecting: "连接中", Live: "监控中", Stopped: "已停止", Failed: "采集失败", Unsupported: "暂不支持采集", Missing: "主机已移除", Updated: "最近更新",
	State: "状态", CPU: "CPU", Memory: "内存", Disk: "磁盘", Network: "网络收发", Receive: "接收", Send: "发送", Terminal: "打开终端", Retry: "重试", PickerHint: "可跨分组选择 SSH 主机。创建后点击“开始监控”才会连接并采集。",
}
var englishDashboard = DashboardMessages{
	View: "View", Metrics: "Resource metrics", Usage: "Usage",
	Host: "Host", Mount: "Mount", Interface: "Interface", Used: "Used", Total: "Total", Available: "Available", ReceivedTotal: "Total received", SentTotal: "Total sent",
	Title: "Monitor", New: "New panel", Edit: "Edit current panel", Delete: "Delete current panel", Start: "Start monitoring", Stop: "Stop monitoring", Simple: "Simple", Detailed: "Detailed", Search: "Search hosts, addresses or tags",
	Name: "Panel name", Note: "Note (optional)", Hosts: "Select hosts", HostSearch: "Search name, address or group", Create: "Create panel", Save: "Save changes", Empty: "Create your first monitoring panel", EmptyHint: "Choose hosts, then start monitoring to see their resource usage.",
	NameRequired: "Enter a panel name (up to 40 characters)", DuplicateName: "A panel with this name already exists", HostsRequired: "Select at least one SSH host", DeletePrompt: "Delete this monitoring panel? Collection will stop and its history will be cleared.",
	Sampling: "Sample", Retention: "Retain", Every: "Every %ds", Minutes: "%d min", Hours: "%d h", Count: "%d hosts", Collecting: "%d / %d hosts collecting",
	Waiting: "Not started", Connecting: "Connecting", Live: "Monitoring", Stopped: "Stopped", Failed: "Collection failed", Unsupported: "Unsupported", Missing: "Host removed", Updated: "Updated",
	State: "Status", CPU: "CPU", Memory: "Memory", Disk: "Disk", Network: "Network", Receive: "Receive", Send: "Send", Terminal: "Open terminal", Retry: "Retry", PickerHint: "Select SSH hosts across groups. Creating a panel does not connect; use Start monitoring.",
}
