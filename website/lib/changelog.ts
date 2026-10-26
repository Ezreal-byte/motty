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
