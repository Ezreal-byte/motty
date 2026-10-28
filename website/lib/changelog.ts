/**
 * The changelog, newest first: one entry per release, written when the release
 * is published (docs/release.md). Both languages live next to each other so a
 * release does not need a separate docs translation pass, and the page stays a
 * static export — nothing is fetched at build or run time.
 */
export type ChangelogText = { en: string; zh: string };

export type ChangelogSection = {
  title: ChangelogText;
  items: ChangelogText[];
};

export type ChangelogEntry = {
  /** Version without the v; the release tag is `v${version}`. */
  version: string;
  /** Publish date, ISO 8601. */
  date: string;
  sections: ChangelogSection[];
};

export const changelog: ChangelogEntry[] = [
  {
    version: '0.1.7',
    date: '2026-10-28',
    sections: [
      {
        title: { en: 'New', zh: '新功能' },
        items: [
          {
            en: 'Snippets have their own page in the sidebar, below Known Hosts: a table of each command’s name, shortcut and text, with add, edit and delete. Every snippet can have a shortcut of its own, set by pressing the keys in its dialog. It cannot collide with a built-in shortcut or another snippet, Control alone needs Shift or Alt so keys like Ctrl+R keep working in the terminal, and it only sends while the terminal page is showing. Shortcuts stay on this device.',
            zh: '侧边栏「已知主机」下面新增「快捷命令」页面，用表格管理每条命令的名称、快捷键和内容，可添加、编辑、删除。每条命令都能绑定自己的快捷键，在编辑对话框里按下按键即可设置：不会和内置快捷键或其他快捷命令重复，只用 Control 时需再加 Shift 或 Alt，免得占用 Ctrl+R 这类终端自带按键，并且只在终端页显示时发送。快捷键只保存在本机。',
          },
          {
            en: 'Connection settings choose the ssh-agent: the system agent, 1Password, Bitwarden, or a custom socket or named pipe, kept per device. Windows connects to the OpenSSH pipe by default, and an agent that cannot be reached is listed as skipped with its address.',
            zh: '连接设置里可以选择 ssh-agent：系统 agent、1Password、Bitwarden，或自定义 socket / 命名管道，按设备保存。Windows 默认连接 OpenSSH 命名管道；所选 agent 连不上时，会在跳过列表里写明地址。',
          },
          {
            en: 'Keychain can save the passphrase of an encrypted key (Save Passphrase), checked against the key first and kept in the vault; saving it empty forgets it.',
            zh: '密钥链里加密私钥新增「保存口令」：先用口令校验密钥，再存入保险库；保存为空即清除。',
          },
          {
            en: 'Pasting in an SSH terminal while the clipboard holds an image and no text uploads the picture to ~/.cache/motty/paste on the server (SFTP, or SCP when there is no SFTP) and types its path, so programs such as Claude Code can read it. Files are private to the account, uploads older than a week are pruned, and a setting turns it off.',
            zh: '在 SSH 终端里粘贴时，如果剪贴板里是图片而没有文本，会通过 SFTP（没有 SFTP 时用 SCP）把图片传到服务器的 ~/.cache/motty/paste 并输入路径，方便 Claude Code 之类的程序读取。文件只有当前账号可读写，超过一周的上传会被清理，设置里可以关闭。',
          },
          {
            en: 'The terminal follows the window between light and dark, and each look remembers its own color scheme.',
            zh: '终端会跟随窗口在浅色和深色之间切换，两种外观各记住自己的配色。',
          },
          {
            en: 'RDP hosts have a switch to connect to the console (admin) session.',
            zh: '远程桌面主机表单新增「连接到控制台（管理）会话」开关。',
          },
        ],
      },
      {
        title: { en: 'Improved', zh: '改进' },
        items: [
          {
            en: 'Private keys that begin with ENCRYPTED PRIVATE KEY (the default of OpenSSL 3 and ssh-keygen -m PKCS8) now open and ask for the passphrase like any other encrypted key. A key file that exists but cannot be used is listed as skipped with its reason.',
            zh: '以 ENCRYPTED PRIVATE KEY 开头的私钥（OpenSSL 3 和 ssh-keygen -m PKCS8 的默认格式）现在可以打开，并像其他加密密钥一样询问口令；存在但无法使用的密钥文件会连同原因出现在跳过列表里。',
          },
          {
            en: 'The files panel of a local terminal opens at, and follows, the folder the shell is in (local bash and zsh; Windows shells are not covered).',
            zh: '本地终端的文件面板会打开并跟随 shell 所在的目录（本地 Bash / Zsh；Windows shell 暂不支持）。',
          },
          {
            en: 'Programs that copy on selection (Claude Code, tmux set-clipboard) no longer raise a prompt on every drag: OSC 52 writes are allowed, reads stay denied, and the ask dialog gains Always allow.',
            zh: '选中即复制的程序（Claude Code、tmux 的 set-clipboard）不再每次拖选都弹窗：OSC 52 写剪贴板直接允许，读仍然拒绝，询问对话框里新增「始终允许」。',
          },
        ],
      },
      {
        title: { en: 'Fixed', zh: '修复' },
        items: [
          {
            en: 'With a files panel open beside a terminal, the part of the terminal as far from its left edge as the panel is wide no longer swallows the mouse wheel.',
            zh: '终端旁边打开文件面板时，终端里离左边缘与面板同宽的那一段不再吞掉滚轮。',
          },
          {
            en: 'Reopening or restoring the remote tools window reads the active tab again, so a container created later no longer looks deleted.',
            zh: '远程工具窗口重新打开或恢复时会重新读取当前标签，之后新建的容器不再显示成已删除。',
          },
          {
            en: 'The chosen terminal color scheme is marked with a check; a dark scheme in a light window used to look unselected.',
            zh: '选中的终端配色现在有对勾标记；深色配色在浅色窗口里原先看不出选中。',
          },
          {
            en: 'Clicking outside the key passphrase prompt no longer skips that key, and a key whose fingerprint is unknown until unlocked can no longer be pinned to a host or block removing the key.',
            zh: '点在口令提示框外不再误把这把密钥跳过；未解锁前指纹未知的密钥不能再固定到某台主机，也不再阻止删除该密钥。',
          },
        ],
      },
    ],
  },
  {
    version: '0.1.6',
    date: '2026-10-28',
    sections: [
      {
        title: { en: 'New', zh: '新功能' },
        items: [
          {
            en: 'The file panel follows the folder the SSH terminal is in by default. New bash and zsh sessions report their folder for that session only, without touching the server’s startup files. Settings > Files also has a separate “Terminal follows file navigation” switch, off by default, which changes the same terminal’s folder at a verified empty prompt and otherwise says why it did not.',
            zh: '文件面板默认跟随 SSH 终端所在目录；新连接的 Bash / Zsh 会安装仅本次会话生效的目录上报，不改动服务器上的启动文件。设置 > 文件里另有默认关闭的「终端跟随文件栏目录」：开启后进入远程文件夹也会切换同一个终端的目录，只在验证过的空提示符下同步，其余情况给出提示。',
          },
          {
            en: 'The host editor picks an existing connection group or creates one from the name you type.',
            zh: '编辑主机时，分组一栏可以直接选已有分组，也能输入新名称创建。',
          },
          {
            en: 'Settings > Terminal > Local shell chooses an installed shell for new or reconnected local terminals on Windows, picked from what Motty finds on PATH and in the usual install folders.',
            zh: 'Windows 下设置 > 终端新增「本地 Shell」，列出从 PATH 和常见安装目录检测到的 PowerShell、CMD、Git Bash 等，用作新建或重连的本地终端。',
          },
          {
            en: 'WebDAV and S3 backups can use a remote HTTP address after an explicit confirmation. HTTPS stays the default, the form keeps warning that HTTP is not encrypted, and the backup is still encrypted on this computer before upload.',
            zh: '局域网里的 WebDAV / S3 服务可以在表单底部勾选「允许不安全的远程 HTTP」，确认后才保存，并一直显示不加密的风险；备份文件仍然在本机加密后上传。',
          },
        ],
      },
      {
        title: { en: 'Improvements', zh: '改进' },
        items: [
          {
            en: 'The main window, file windows and remote tool windows keep a usable minimum size instead of letting controls be cut off.',
            zh: '窗口、独立文件窗口和远程工具窗口都有可用的最小尺寸，缩到极限时控件不再被裁掉。',
          },
          {
            en: 'The docked assistant panel can be dragged wider: its width is bounded only by the room the content beside it keeps, with the navigation rail counted as part of that room.',
            zh: '停靠的 AI 面板可以拉得更宽：宽度只受旁边内容保留的空间限制，状态栏导航也算作保留空间。',
          },
          {
            en: 'In narrow windows, section toolbars turn into labelled icon buttons when they do not fit, host rows give up their status words but keep the light, and titles wrap or ellipsize instead of running under the buttons.',
            zh: '窄窗口里，主机、密钥、代理等页的工具栏按钮放不下时变成带提示的图标，主机行让出状态文字、保留指示灯，标题会换行或省略，不再压到按钮下面。',
          },
          {
            en: 'Assistant CLI settings probe what the installed CLI can do instead of checking versions: they show the path found or say none was found, and name the missing capability, the failed check command or an isolation setting that did not apply.',
            zh: 'AI CLI 设置改为实际探测能力：会显示找到的可执行文件路径或提示未找到，并分别说明缺少哪项能力、检查命令失败还是隔离设置没生效。',
          },
        ],
      },
      {
        title: { en: 'Fixes', zh: '修复' },
        items: [
          {
            en: 'A server that only allows SFTP keeps its connection when the shell is refused, and the file panel keeps working. A connection also stays while a file panel or file window still uses it after the shell ends.',
            zh: '只允许 SFTP 的服务器不再因为 shell 被拒绝而断开连接，文件面板继续可用；用户结束 shell 时，只要文件面板或独立文件窗口还在使用该连接，连接就留到最后一个窗口关闭。',
          },
          {
            en: 'The file panel breadcrumbs come back when the path input loses focus.',
            zh: '文件面板的路径输入框失去焦点后，面包屑会恢复显示。',
          },
          {
            en: 'Remote process lists parse the fields of older procps again.',
            zh: '服务器上的 procps 较旧、进程字段不同时，远程进程列表不再解析失败。',
          },
          {
            en: 'Folder reports that arrive while the file panel is opening, busy or editing its path no longer move it; only the latest one is applied.',
            zh: '文件面板正在打开、忙碌或正在编辑路径时，迟到的目录上报不会让列表跳走，只会应用最新的那一次。',
          },
        ],
      },
    ],
  },
  {
    version: '0.1.5',
    date: '2026-10-28',
    sections: [
      {
        title: { en: 'New', zh: '新功能' },
        items: [
          {
            en: 'Settings > General customizes the window, terminal and SFTP toolbars: which buttons are shown and in what order, dragged or moved with the keyboard, applied immediately, with enable all, disable all and restore defaults.',
            zh: '设置 > 通用可以自定义窗口、终端和 SFTP 工具栏：显示哪些按钮、按什么顺序，拖动或用方向键排序，改动立即生效，并可全选、全不选、恢复默认。',
          },
          {
            en: 'A connection’s color now marks its host card and its tab, and the file window header uses the same color as the terminal.',
            zh: '连接的颜色现在出现在主机卡片和标签页上，文件窗口头部也和终端一致。',
          },
          {
            en: 'A file panel can filter the current folder by name with a match count, show or hide hidden files, and edit its path directly with Command/Ctrl+L.',
            zh: '文件面板能按名字筛选当前目录并显示命中数，可以切换隐藏文件，Command/Ctrl+L 直接编辑路径。',
          },
          {
            en: 'Settings > MCP separates the HTTP server, which uses the sessions open in Motty, from the standalone service a client starts (`mcp-serve`, over the hosts you opened to agents); each has its own client list, preview and copy button.',
            zh: '设置 > MCP 把 HTTP 服务（用窗口里打开的会话）和客户端启动的独立服务（`mcp-serve`，用已授权主机）分开，各有自己的客户端、预览和复制按钮。',
          },
          {
            en: 'Both transports can grant full access: commands, reads, writes and deletes run without approval. It is saved only after a risk confirmation, stays flagged while configured, and for the standalone service adds `sftp_write` and `sftp_remove` and takes effect when the client restarts it.',
            zh: '两种接入都能开完全权限：执行命令、读写删除文件都不再逐次审批。选择时先风险确认，配置期间持续标红；独立服务下还提供 `sftp_write`、`sftp_remove`，要等客户端重启服务才生效。',
          },
          {
            en: 'The assistant keeps its primary connection when you switch tabs; acting on another session needs an @ reference or an explicit request, and cross-connection commands and file changes show their real target for approval.',
            zh: 'AI 助手的主连接不随标签页切换而改变；操作别的会话要用 @ 或明确要求，跨连接的命令和文件改动在审批时会显示真实目标。',
          },
          {
            en: 'The Docker view says when Docker is not installed, sorts by column, and can refresh every 5 seconds to 5 minutes.',
            zh: 'Docker 页面在服务器没装 Docker 时给出空状态和提示，列可以点标题排序，自动刷新间隔从 5 秒到 5 分钟。',
          },
          {
            en: 'Release notes in the update dialog are rendered Markdown, with clickable links.',
            zh: '更新对话框里的 release notes 改为渲染后的 Markdown，链接可以点开。',
          },
        ],
      },
      {
        title: { en: 'Improved', zh: '改进' },
        items: [
          {
            en: 'The Ask level is spelled out: low-risk actions run, the rest need approval in the AI panel, and critical ones need a typed confirmation.',
            zh: '「询问」级别的含义写清楚了：低风险操作自动执行，其余在 AI 助手面板批准，极高风险操作需要输入确认文字。',
          },
          {
            en: 'Custom Windows install directories update in place now, keeping the original directory; an older build that could only offer a download has to be replaced by hand once.',
            zh: '自定义安装目录的 Windows 版本也能原地更新并沿用原目录；之前只能拿浏览器下载的旧版本需要手动装一次这一版。',
          },
          {
            en: 'On Apple Silicon under Rosetta the update installs the arm64 package.',
            zh: 'Apple Silicon 上以 Rosetta 运行更新时，装的是 arm64 包。',
          },
          {
            en: 'The status icon is created after the first window is up.',
            zh: '状态图标在第一个窗口就绪之后才创建。',
          },
        ],
      },
      {
        title: { en: 'Fixed', zh: '修复' },
        items: [
          {
            en: 'Page content stays inside the workspace border while scrolling.',
            zh: '页面滚动时内容不再越出工作区边框。',
          },
          {
            en: 'Transfer progress and results stay in the detached file window’s own footer.',
            zh: '独立文件窗口里，传输的进度和结果留在它自己的底部。',
          },
          { en: 'The assistant drawer no longer sits over dialogs.', zh: 'AI 助手抽屉不再压到对话框上。' },
        ],
      },
    ],
  },
  {
    version: '0.1.4',
    date: '2026-10-28',
    sections: [
      {
        title: { en: 'New', zh: '新功能' },
        items: [
          {
            en: '`motty mcp-serve` is an MCP server over stdio: an agent starts it itself, with no port and no token, and it ends with that agent. It reaches only the hosts you opened to agents under a host’s Advanced settings.',
            zh: '`motty mcp-serve` 是走 stdio 的 MCP 服务：agent 自己启动它，不用端口也不用 token，agent 退出它就结束；它只访问你在主机「高级」里开放给 agent 的主机。',
          },
          {
            en: 'Agents are read only by default; the Commands mode also allows commands rated low risk. Anything that needs approval is refused, host keys are never trusted by the server itself, and passwords come from the saved passwords.',
            zh: 'agent 默认只读，「命令」档额外放行低风险命令；需要确认的操作一律拒绝，服务不会自行信任主机密钥，密码取自已保存的密码。',
          },
          {
            en: 'Every call is listed under Settings > MCP for 30 days, without its output; the server refuses to start when it cannot open that log. Settings > MCP also carries copy-ready entries for the supported clients.',
            zh: '每次调用都会列在设置 > MCP，保留 30 天且不含输出，日志打不开时服务拒绝启动；设置 > MCP 里还有可直接复制的客户端配置片段。',
          },
          {
            en: 'A GitHub button in the window bar opens the project page. The language button is gone; the language still lives in Settings.',
            zh: '窗口栏新增 GitHub 按钮，点开是项目主页；语言按钮去掉，语言仍在设置里选。',
          },
        ],
      },
      {
        title: { en: 'Improved', zh: '改进' },
        items: [
          {
            en: 'Advanced host settings are grouped under Session and AI agents, name and color share a row, the agent switch explains itself in a help tooltip, and the proxy summary is a bordered box like the other inputs.',
            zh: '主机高级设置按「会话」和「AI agent」分组，名称和颜色排在同一行，agent 开关带帮助提示，代理摘要改成和其他输入一致的有边框盒子。',
          },
          {
            en: 'iTerm2, kitty and sixel images decode in the background, so a large image no longer stalls a frame; the placement is reserved while decoding, and erases, resizes and kitty deletes still see it.',
            zh: 'iTerm2、kitty、sixel 图片改到后台解码，大图不再卡住一帧；解码期间位置已经占好，擦除、缩放和 kitty 删除照常生效。',
          },
          {
            en: 'Hovering a terminal link scans only the soft-wrapped group under the pointer instead of the whole scrollback, and saving one host re-encodes only that host’s record.',
            zh: '悬停终端链接只扫描指针所在的那一组软换行，不再整屏回扫；保存一台主机只重新编码那条记录，主机很多时也快。',
          },
        ],
      },
      {
        title: { en: 'Fixed', zh: '修复' },
        items: [
          {
            en: 'The MCP group in Settings no longer collides with the cards around it, and its title and descriptions say what the user cares about.',
            zh: '设置里的 MCP 分组不再和周围的卡片挤在一起，标题和说明改成用户关心的说法。',
          },
        ],
      },
    ],
  },
  {
    version: '0.1.3',
    date: '2026-10-27',
    sections: [
      {
        title: { en: 'New', zh: '新功能' },
        items: [
          {
            en: 'SFTP opens in its own window with the local and remote panes side by side; rows sort, drop onto each other, and a pane can connect straight to a host.',
            zh: 'SFTP 可以在自己的窗口里并排打开本地和远程两栏，行可排序、互相拖放，也能从主机直连。',
          },
          {
            en: 'A global transfer center lists every upload and download with its progress and lifetime, and keeps going after the window that started it closes. The file panel shows the transfer list by default and docks left or right, and its path bar is editable with a choice of style.',
            zh: '全局传输中心列出每个上传下载的进度和生命周期，发起它的窗口关掉后任务照常走完。文件面板默认显示传输列表，可停靠左侧或右侧，路径栏可编辑并能切换样式。',
          },
          {
            en: 'Compress a remote folder into a zip on the server itself, and extract more archive formats with a notice of how it went.',
            zh: '远程文件夹直接在服务器上打包成 zip，解压支持更多归档格式，并给出结果提示。',
          },
          {
            en: 'Preview images and edit text in a native window: the encoding is detected, and the main window stays free.',
            zh: '图片在独立窗口里预览，文本按识别出的编码编辑，主窗口不被占用。',
          },
          {
            en: 'Remote tools open in their own native window and run alongside the main window; legacy Docker and ps output is understood, and a container gets its own terminal.',
            zh: '远程工具开在自己的原生窗口里，和主窗口并行；兼容旧版 Docker 和 ps 的输出，进入容器时用它自己的终端。',
          },
          {
            en: 'Each host can choose its own authentication methods and its own SOCKS5 or HTTP CONNECT proxy, credentials included.',
            zh: '每台主机可以单独选择认证方式，也可以单独配 SOCKS5 或 HTTP CONNECT 代理，含用户名密码。',
          },
          {
            en: 'A connection can carry its own color into the terminal header and tool buttons, picked with a round eyedropper in the form; the host form rows got roomier.',
            zh: '每个连接可以带一种颜色，出现在终端顶栏和工具按钮上，表单里用圆形吸管取色；主机表单的行距也调宽了。',
          },
          {
            en: 'Host tags are a searchable multi-select that can create an entry on the spot.',
            zh: '主机标签改成可搜索、可现场新建条目的多选框。',
          },
          {
            en: 'Import model providers from magpie and cc-switch, drawn with their official marks.',
            zh: '可以从 magpie 和 cc-switch 导入模型供应商，并使用它们的官方标识。',
          },
          {
            en: 'Close a split pane from the terminal menu and the keyboard follows the pane you enter; the command palette keeps the highlighted row under the pointer and takes arrow keys in its search field.',
            zh: '终端菜单里可以关闭分屏，键盘跟着你进入的分栏走；命令面板在鼠标悬浮时保留选中项，搜索框里可以用方向键移动。',
          },
        ],
      },
      {
        title: { en: 'Improved', zh: '改进' },
        items: [
          {
            en: 'Every pull request runs the full Go checks, and the race detector is sharded with cached builds so the gate finishes sooner.',
            zh: '每个 PR 都跑完整的 Go 检查，race 检测拆成分片并缓存构建，门禁更快也更稳。',
          },
          { en: 'The debug gallery gained a color swatch.', zh: '调试画廊里加了颜色样本。' },
        ],
      },
      {
        title: { en: 'Fixed', zh: '修复' },
        items: [
          {
            en: 'The local file panel no longer reports an SFTP EOF when it enters an empty directory.',
            zh: '本地文件面板进入空目录时不再误报 SFTP EOF。',
          },
          {
            en: 'A restored tab starts disconnected instead of reconnecting on its own.',
            zh: '恢复的标签页默认不再自动重连。',
          },
          { en: 'Set Model in the AI panel opens the AI settings page.', zh: 'AI 面板里的「设置模型」跳到 AI 设置页。' },
          {
            en: 'The command palette, overlay close buttons and the new-tab picker take their clicks and keys where they are drawn.',
            zh: '命令面板、overlay 关闭按钮和新标签页选择器的点击与按键落在它画出来的位置。',
          },
          {
            en: 'An RDP server can no longer make the client hold unbounded pixel memory.',
            zh: 'RDP 服务端不能再让客户端持有无上限的像素内存。',
          },
          {
            en: 'The macOS update helper runs from a full copy of the bundle, so the replacement lands reliably.',
            zh: 'macOS 更新助手从完整的 bundle 副本运行，替换更可靠。',
          },
        ],
      },
    ],
  },
  {
    version: '0.1.2',
    date: '2026-10-27',
    sections: [
      {
        title: { en: 'New', zh: '新功能' },
        items: [
          {
            en: 'An About page in Settings shows this copy\'s version, platform, build commit and runtime, with one button that copies them for a bug report, and links to the community, the repository, the issue tracker and motty.page.',
            zh: '设置里新增「关于」，显示这一份 Motty 的版本、平台、构建 commit 和运行时，一个按钮把它们复制下来贴进 issue，并提供社区、仓库、issue 和 motty.page 的入口。',
          },
          {
            en: 'Hosts now sync both ways with ~/.ssh/config: host, user, port and ProxyJump travel either direction and IdentityFile lands in Keys. The file keeps its comments and unknown directives, a block naming several hosts is left alone, and when both sides changed the file wins.',
            zh: '主机与 ~/.ssh/config 双向同步：host、user、port、ProxyJump 两个方向都同步，IdentityFile 落到 Keys。文件保留注释和未知指令，一条 Host 行带多个主机名的块不动，两边都改时以文件为准。',
          },
          {
            en: 'Select text inside an AI answer and copy it; a fenced block keeps its own line breaks and indentation.',
            zh: 'AI 回答里的文字可以选中并复制，代码块保留原有的换行和缩进。',
          },
          {
            en: 'Command/Control+Shift+T opens a local terminal.',
            zh: 'Command/Control+Shift+T 打开本地终端。',
          },
          { en: 'Check for Updates… in the status-area menu.', zh: '状态栏菜单里的「检查更新…」。' },
        ],
      },
      {
        title: { en: 'Improved', zh: '改进' },
        items: [
          {
            en: 'Updates are checked five seconds after a window wakes and then once an hour, on by default.',
            zh: '窗口唤醒 5 秒后检查一次更新，之后每小时一次，默认开启。',
          },
          {
            en: 'An install the system owns (a distribution package, /snap, Flatpak, Program Files) is told to update through its package manager or the new installer, and is still never replaced by the app with elevation.',
            zh: '系统持有的安装（发行版包、/snap、Flatpak、Program Files）会被告知用包管理器或新安装包更新，应用仍然不会提权替换它。',
          },
          {
            en: 'Every package carries a minisign update signature that names its version and file, and the app verifies it before replacing anything. Windows can now update in place without a code-signing certificate; macOS still checks the Developer ID team.',
            zh: '每个包都带 minisign 更新签名，签名里写明版本和文件名，应用在替换任何东西之前先校验。Windows 不再需要代码签名证书也能原地更新；macOS 仍校验 Developer ID 团队。',
          },
          {
            en: 'The component gallery is a development-only surface and no longer appears in release builds.',
            zh: '组件画廊只在开发构建里出现，发布构建不再包含它。',
          },
        ],
      },
      {
        title: { en: 'Fixed', zh: '修复' },
        items: [
          {
            en: 'A UTF-8 control string could leak its continuation bytes into the input line as text. Raw eight-bit C1 DCS/ST/OSC/APC are no longer interpreted as controls; use their seven-bit ESC forms.',
            zh: 'UTF-8 控制字符串里的续字节此前会漏进输入行变成文字。原始的八位 C1 DCS/ST/OSC/APC 现在不再被当作控制符解释，请改用七位 ESC 形式。',
          },
          { en: 'The MCP configuration preview names itself in its header.', zh: 'MCP 配置预览的头部写明它的名称。' },
        ],
      },
    ],
  },
  {
    version: '0.1.1',
    date: '2026-10-26',
    sections: [
      {
        title: { en: 'New', zh: '新功能' },
        items: [
          {
            en: 'Motty updates itself: it downloads the release from the mirror, checks its SHA-256 and signature, and restarts into the new version. Installs it does not own (a .deb, Program Files, a portable copy) still open the download page.',
            zh: 'Motty 可以自己更新了：从镜像下载安装包，校验 SHA-256 和签名后重启进入新版本。不是自己持有的安装（.deb、Program Files、便携版）仍会打开下载页。',
          },
          {
            en: 'Local Claude Code, Codex and OpenCode CLI providers, each with its own login terminal.',
            zh: '本地 Claude Code、Codex、OpenCode CLI 提供方，各自带登录终端。',
          },
          {
            en: 'A per-host outbound proxy: inherit, direct, SOCKS5 or HTTP CONNECT.',
            zh: '每台主机可单独设置出站代理：继承、直连、SOCKS5 或 HTTP CONNECT。',
          },
          {
            en: 'Copy the MCP client configuration for 13 clients from Settings.',
            zh: '在设置里复制 13 种客户端的 MCP 配置。',
          },
          { en: 'Import Electerm bookmarks with their nested groups.', zh: '导入 Electerm 书签及其嵌套分组。' },
          {
            en: 'Save a host password when creating or copying a host; hosts with the same endpoint can share it.',
            zh: '新建或复制主机时保存密码，同一地址的主机可以共用。',
          },
          { en: 'Duplicate a tab and close a pane from the interface.', zh: '界面里复制标签页、关闭分屏。' },
        ],
      },
      {
        title: { en: 'Fixed', zh: '修复' },
        items: [
          { en: 'Ctrl+C reaches a terminal without a selection again.', zh: '终端没有选中内容时也能收到 Ctrl+C。' },
          {
            en: 'AI rename dialogs take their buttons again, and the wheel no longer scrolls the panel behind them.',
            zh: 'AI 重命名对话框的按钮恢复可点，滚轮不再滚动背后的面板。',
          },
          {
            en: 'The host form keeps its fields on one grid in a narrow window.',
            zh: '窄窗口下主机表单的字段保持在同一栅格。',
          },
        ],
      },
    ],
  },
  {
    version: '0.1.0',
    date: '2026-10-26',
    sections: [
      {
        title: { en: 'New', zh: '新功能' },
        items: [
          { en: 'Native SSH terminal with tabs and split panes.', zh: '原生 SSH 终端，支持标签页和分屏。' },
          {
            en: 'Hosts, groups and tags, with password or key authentication and jump chains.',
            zh: '主机、分组、标签，支持密码或密钥认证以及跳板链。',
          },
          {
            en: 'An SFTP browser beside the terminal: upload, download, rename and delete.',
            zh: '终端旁的 SFTP 文件浏览：上传、下载、重命名、删除。',
          },
          {
            en: 'Port forwarding managed in the interface: local, remote and dynamic.',
            zh: '界面里管理端口转发：本地、远程、动态。',
          },
          { en: 'VNC and RDP desktops in the same window.', zh: '同一个窗口里的 VNC 和 RDP 桌面。' },
          {
            en: 'An AI assistant and an MCP server that work on the session in front of you.',
            zh: 'AI 助手与 MCP 服务，作用于当前会话。',
          },
        ],
      },
      {
        title: { en: 'Packaging', zh: '安装' },
        items: [
          {
            en: 'Packages for macOS (arm64, amd64), Windows (amd64, arm64) and Linux (amd64, arm64). The macOS builds are signed, notarized and stapled; the Windows and Linux builds are unsigned.',
            zh: 'macOS（arm64、amd64）、Windows（amd64、arm64）、Linux（amd64、arm64）安装包。macOS 已签名、公证并 staple；Windows 和 Linux 尚未签名。',
          },
        ],
      },
    ],
  },
];

export const changelogPage: Record<
  'en' | 'zh',
  {
    title: string;
    description: string;
    versions: string;
    publishedOn: string;
    download: string;
    tocSubtitle: string;
    /** `In v0.1.0` above the section list of the release in view. */
    tocTitle: (version: string) => string;
  }
> = {
  en: {
    title: 'Changelog',
    description: 'Track every release: features, improvements and fixes.',
    versions: 'Versions',
    publishedOn: 'Published on',
    download: 'Download',
    tocSubtitle: 'Release contents',
    tocTitle: (version) => `In ${version}`,
  },
  zh: {
    title: '更新日志',
    description: '追踪每次发布：新功能、改进和修复。',
    versions: '版本列表',
    publishedOn: '发布于',
    download: '下载',
    tocSubtitle: '更新内容',
    tocTitle: (version) => `在 ${version} 中`,
  },
};

/** `v0.1.0`, the way the tag and the release title spell it. */
export function versionLabel(version: string): string {
  return `v${version}`;
}

/** Anchor ids in the same shape the DBX changelog uses. */
export function releaseId(version: string): string {
  return `changelog-release-v${version.replace(/\./g, '-')}`;
}

export function sectionId(version: string, index: number): string {
  return `${releaseId(version)}-section-${index}`;
}
