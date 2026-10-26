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
  { title: string; description: string; intro: string; download: string; notes: string }
> = {
  en: {
    title: 'Changelog',
    description: 'Everything that shipped in each Motty release, newest first.',
    intro: 'Everything that shipped in each release, newest first.',
    download: 'Download',
    notes: 'Release notes on GitHub',
  },
  zh: {
    title: '更新日志',
    description: 'Motty 每个版本都带来了什么，最新的在最前面。',
    intro: '每个版本带来的变化，最新的在最前面。',
    download: '下载',
    notes: 'GitHub 上的发布说明',
  },
};

/** `v0.1.0` for display, and an anchor id that is safe in a URL. */
export function versionLabel(version: string): string {
  return `v${version}`;
}

export function versionId(version: string): string {
  return `v${version.replace(/\./g, '-')}`;
}
