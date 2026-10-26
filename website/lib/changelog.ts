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
