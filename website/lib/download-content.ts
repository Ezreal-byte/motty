import { site } from '@/lib/seo';

/**
 * The /download page. The links point at the R2 mirror (tools/mirror-release.sh
 * writes `latest/<stable-name>` for every release), so they never need a site
 * rebuild per version; the version-less names are what the mirror keeps current.
 */
export type DownloadFile = {
  label: string;
  /** The architecture or form factor, shown after the label. */
  kind: string;
  /** `latest/<stable>` under the mirror host. */
  key: string;
};

export type DownloadContent = {
  title: string;
  description: string;
  intro: string;
  groups: { platform: string; files: DownloadFile[] }[];
  checksums: string;
  manifest: string;
  checksumsNote: string;
  sourceNote: string;
  sourceLabel: string;
  install: string;
};

const latest = (key: string) => `${site.download}/latest/${key}`;

const macos: DownloadFile[] = [
  { label: 'Disk image', kind: 'arm64', key: 'motty-darwin-arm64.dmg' },
  { label: 'Disk image', kind: 'amd64', key: 'motty-darwin-amd64.dmg' },
  { label: 'Zip', kind: 'arm64', key: 'motty-darwin-arm64.zip' },
  { label: 'Zip', kind: 'amd64', key: 'motty-darwin-amd64.zip' },
];

const windows: DownloadFile[] = [
  { label: 'Installer', kind: 'amd64', key: 'motty-windows-amd64-setup.exe' },
  { label: 'Zip', kind: 'amd64', key: 'motty-windows-amd64.zip' },
  { label: 'Zip', kind: 'arm64', key: 'motty-windows-arm64.zip' },
];

const linux: DownloadFile[] = [
  { label: 'Debian package', kind: 'amd64', key: 'motty_amd64.deb' },
  { label: 'Debian package', kind: 'arm64', key: 'motty_arm64.deb' },
  { label: 'Tarball', kind: 'amd64', key: 'motty-linux-amd64.tar.gz' },
  { label: 'Tarball', kind: 'arm64', key: 'motty-linux-arm64.tar.gz' },
];

export const download: Record<'en' | 'zh', DownloadContent> = {
  en: {
    title: 'Download Motty',
    description:
      'Packages for macOS, Windows and Linux, served from Cloudflare R2. GitHub Releases stays the source of record.',
    intro:
      'Packages for macOS, Windows and Linux. The links below always point at the current version and are served from Cloudflare R2, so the download does not have to cross a slow link. Every release is also kept, unchanged, on GitHub.',
    groups: [
      { platform: 'macOS', files: macos },
      { platform: 'Windows', files: windows },
      { platform: 'Linux', files: linux },
    ],
    checksums: 'SHA256SUMS',
    manifest: 'latest.json',
    checksumsNote: 'Check a download with sha256sum -c SHA256SUMS (shasum -a 256 -c on macOS).',
    sourceNote: 'The release itself, its notes and its checksums live on',
    sourceLabel: 'GitHub Releases',
    install: 'Installation notes, signing and updates',
  },
  zh: {
    title: '下载 Motty',
    description: 'macOS、Windows 和 Linux 安装包，由 Cloudflare R2 提供下载。GitHub Releases 仍是源头。',
    intro:
      'macOS、Windows 和 Linux 的安装包。下面的链接始终指向当前版本，由 Cloudflare R2 提供，不用绕远路。每个版本也都原样保留在 GitHub 上。',
    groups: [
      { platform: 'macOS', files: macos },
      { platform: 'Windows', files: windows },
      { platform: 'Linux', files: linux },
    ],
    checksums: 'SHA256SUMS',
    manifest: 'latest.json',
    checksumsNote: '校验下载：sha256sum -c SHA256SUMS（macOS 上用 shasum -a 256 -c）。',
    sourceNote: '版本本身、更新说明和校验文件都在',
    sourceLabel: 'GitHub Releases',
    install: '安装说明、签名与更新',
  },
};

/** Resolve a file to its mirrored URL; the page never builds URLs itself. */
export function downloadHref(file: DownloadFile): string {
  return latest(file.key);
}
