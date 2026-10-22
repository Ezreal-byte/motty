package i18n

// DesktopMessages are the texts of remote-desktop tabs (VNC and RDP): the host
// editor's choices, the bar and status line over the picture, and the page shown
// when the connection ended.
type DesktopMessages struct {
	ProtocolVNC     string
	ProtocolRDP     string
	FieldSSHUser    string
	FieldRDPUser    string
	RDPUserHint     string
	ErrRDPUser      string
	CertNewFmt      string
	CertChangedFmt  string
	StripTLS        string
	ViaSSH          string
	ShareClipboard  string
	VNCPlaintext    string
	TipCapture      string
	TipRelease      string
	Fit             string
	Actual          string
	CtrlAltDel      string
	StripCaptured   string
	StripReleased   string
	StripSizeFmt    string
	StripEncrypted  string
	StripPlain      string
	StripClipboard  string
	Disconnected    string
	DisconnectedFmt string
	Reconnect       string
	ReconnectHint   string
	AccessibleFmt   string
	StatusReleased  string
	StatusCaptured  string
	ErrNoDesktop    string
}

var englishDesktop = DesktopMessages{
	ProtocolVNC:     "VNC",
	ProtocolRDP:     "RDP",
	FieldSSHUser:    "SSH user",
	FieldRDPUser:    "Windows user",
	RDPUserHint:     "user or DOMAIN\\user",
	ErrRDPUser:      "An RDP host needs the Windows user it signs in as.",
	CertNewFmt:      "%s presented a certificate you have not trusted before. Windows creates its own, so it cannot be checked against an authority: compare the fingerprint with the one on the server (its Remote Desktop certificate) before you trust it.",
	CertChangedFmt:  "WARNING: the certificate of %s has CHANGED since you trusted it (it was %s). Windows renews its self-signed certificate by itself from time to time, and a reinstalled server has a new one, but an attacker on the network looks exactly like this. Do not continue unless you know why it changed.",
	StripTLS:        "TLS",
	ViaSSH:          "Connect through SSH (the remote desktop port is reached from the SSH server)",
	ShareClipboard:  "Share the clipboard with the remote desktop",
	VNCPlaintext:    "VNC is not encrypted: the screen and everything you type cross the network in the clear, and its password protection is weak. Connect through SSH when you can.",
	TipCapture:      "The keyboard goes to the remote desktop. Click to keep it in Motty (%s)",
	TipRelease:      "The keyboard stays in Motty. Click to send it to the remote desktop (%s)",
	Fit:             "Fit",
	Actual:          "100%",
	CtrlAltDel:      "Ctrl+Alt+Del",
	StripCaptured:   "Keyboard sent to the desktop · %s releases it",
	StripReleased:   "Keyboard kept by Motty · %s captures it",
	StripSizeFmt:    "%d × %d",
	StripEncrypted:  "Through SSH",
	StripPlain:      "Not encrypted",
	StripClipboard:  "Clipboard shared",
	Disconnected:    "Disconnected",
	DisconnectedFmt: "The remote desktop ended the connection: %s",
	Reconnect:       "Reconnect",
	ReconnectHint:   "Press Enter to reconnect",
	AccessibleFmt:   "Remote desktop %s",
	StatusReleased:  "Keyboard released from the remote desktop.",
	StatusCaptured:  "Keyboard goes to the remote desktop.",
	ErrNoDesktop:    "The connection has no remote desktop.",
}

var chineseDesktop = DesktopMessages{
	ProtocolVNC:     "VNC",
	ProtocolRDP:     "RDP",
	FieldSSHUser:    "SSH 用户",
	FieldRDPUser:    "Windows 用户",
	RDPUserHint:     "用户名或 域\\用户名",
	ErrRDPUser:      "RDP 主机需要填写登录用的 Windows 用户名。",
	CertNewFmt:      "%s 出示了一个你还没有信任过的证书。Windows 自己生成证书，无法用证书颁发机构验证：信任之前请与服务器上的指纹（远程桌面证书）核对。",
	CertChangedFmt:  "警告：%s 的证书自你信任之后已更改（原来是 %s）。Windows 会不时自动更新自签名证书，重装的服务器也会有新证书，但网络上的攻击者看起来也完全一样。不清楚原因时请不要继续。",
	StripTLS:        "TLS",
	ViaSSH:          "通过 SSH 连接(从 SSH 服务器访问远程桌面端口)",
	ShareClipboard:  "与远程桌面共享剪贴板",
	VNCPlaintext:    "VNC 不加密：屏幕内容和你输入的一切都以明文经过网络，其密码保护也很弱。可能的话请通过 SSH 连接。",
	TipCapture:      "键盘现在交给远程桌面。点击改为留在 Motty（%s）",
	TipRelease:      "键盘现在留在 Motty。点击交给远程桌面（%s）",
	Fit:             "适应",
	Actual:          "100%",
	CtrlAltDel:      "Ctrl+Alt+Del",
	StripCaptured:   "键盘已交给桌面 · %s 释放",
	StripReleased:   "键盘留在 Motty · %s 捕获",
	StripSizeFmt:    "%d × %d",
	StripEncrypted:  "经 SSH",
	StripPlain:      "未加密",
	StripClipboard:  "剪贴板已共享",
	Disconnected:    "已断开",
	DisconnectedFmt: "远程桌面结束了连接：%s",
	Reconnect:       "重新连接",
	ReconnectHint:   "按 Enter 重新连接",
	AccessibleFmt:   "远程桌面 %s",
	StatusReleased:  "键盘已从远程桌面释放。",
	StatusCaptured:  "键盘交给远程桌面。",
	ErrNoDesktop:    "该连接没有远程桌面。",
}
