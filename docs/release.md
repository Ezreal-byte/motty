# Releasing Motty

A release is a version tag. Pushing `v0.1.2` runs `.github/workflows/release.yml`,
which builds every package, signs what it has the secrets to sign, and uploads
everything to a **draft** release. You read the draft and publish it.

```sh
# 1. bump version in viva.toml, commit, push the branch
# 2. tag exactly v<version> and push the tag
git tag v0.1.2 && git push origin v0.1.2
# 3. when the workflow is green, review the draft on GitHub and publish it
gh release edit v0.1.2 --draft=false
```

`tools/release.sh` refuses a tag that is not `v` plus the version in `viva.toml`.

## What each job does

| Job | Runs on | Builds |
| --- | --- | --- |
| Pin viva | Ubuntu | resolves `vivaui/viva` main to one commit |
| Draft release | Ubuntu | creates the draft if there is none |
| macOS | macOS 15 (arm64) | `.dmg` and `.zip` for arm64 and amd64; signed, notarized and stapled |
| Linux and Windows | Ubuntu | `.tar.gz` and `.deb` (amd64, arm64), and an NSIS installer for Windows amd64 and arm64; the Windows programs and the installers signed |
| Checksums | Ubuntu | `SHA256SUMS` of everything the release then holds |

The two build jobs run at the same time and upload to the same draft. Re-running
a failed job replaces its files. The Linux packages are not signed.

## The framework commit

Motty is built against the sibling Viva checkout, so a release has to name which
frame it was built on. The *Pin viva* job runs first and asks GitHub for
`vivaui/viva` main's commit; both build jobs then check out that exact commit and
pass it to `tools/release.sh --viva-ref`, and the checksums job writes a
`Built against vivaui/viva <sha>.` line into the release notes. Main's tip at the
start of the release is what gets used: a push to Viva while the release runs
cannot hand the Mac job and the Linux job different frames, and the commit stays
readable from the release afterwards.

A local run uses whatever `../viva` holds unless told otherwise, so pass the same
commit to compare with CI: `tools/release.sh --platforms linux --viva-ref <sha>`.

The macOS runner's architecture does not decide the output architecture: the
release script builds both arm64 and amd64, and Viva cross-builds the cgo/Metal
bridge for the other one. The deployment target is the `[mac] min_system` in
`viva.toml` (default 12.0), not the runner's macOS version. `viva` has a test that
builds a cgo program for the other Mac architecture and checks its Mach-O slice.

## Secrets

All optional: a platform without its secrets is built unsigned and the log says so
in a warning. Set them in the repository's *Settings > Secrets and variables >
Actions*, or with `gh secret set NAME`.

**macOS**

| Secret | What |
| --- | --- |
| `APPLE_CERTIFICATE` | the *Developer ID Application* certificate **with its private key**, exported from Keychain Access as a `.p12`, then `base64 -i certificate.p12` |
| `APPLE_CERTIFICATE_PASSWORD` | the password you gave the `.p12` when exporting |
| `APPLE_ID`, `APPLE_PASSWORD`, `APPLE_TEAM_ID` | your Apple ID, an app-specific password for it (account.apple.com > Sign-In and Security) and the team ID (the `(TEAMID)` in the identity's name) |
| `APPLE_API_KEY_P8`, `APPLE_API_KEY_ID`, `APPLE_API_ISSUER` | instead of the three above: an App Store Connect API key (the contents of the `.p8`, its key ID and the issuer ID). Easier to revoke on its own |

The certificate is imported into a keychain created for the job and deleted with
it; no other keychain is touched. A certificate without notarization credentials
fails the job rather than shipping an image Gatekeeper would block.

**Windows**

| Secret | What |
| --- | --- |
| `WINDOWS_CERTIFICATE`, `WINDOWS_CERTIFICATE_PASSWORD` | a code signing certificate as a `.pfx` (`base64 -i code-signing.pfx`) and its password |
| `WINDOWS_SIGN_COMMAND` | instead: a command that signs `{file}` in place, for a key that lives in a token or a service such as Azure Trusted Signing. It runs on the Ubuntu runner, which has no `signtool`, so it must be something that runs there |

Current certificates mostly keep the key where a file cannot reach it; check what
your certificate authority issues before buying one. Packages are timestamped by
DigiCert's server unless `WINDOWS_TIMESTAMP_URL` says otherwise.

**Viva**

Motty builds against the `main` branch of `vivaui/viva`, checked out beside it. If that
repository is private, add `REED_REPO_TOKEN`, a token that can read it. Because
`main` moves, a rebuild of an old tag can differ from the original build; tag only
when Viva's `main` is what you want to ship.

## Building by hand

```sh
tools/release.sh                       # everything this machine can build, in build/release-<version>
tools/release.sh --platforms linux,windows --out /tmp/out
APPLE_SIGNING_IDENTITY="Developer ID Application: Name (TEAMID)" \
APPLE_NOTARY_PROFILE=motty tools/release.sh --platforms mac
```

On a Mac that is signed in to a keychain with the certificate, `APPLE_SIGNING_IDENTITY`
and a profile made once with `xcrun notarytool store-credentials motty` are enough;
no `.p12` is needed. See `viva help package` for every variable.

## The changelog page

`website/lib/changelog.ts` holds one entry per release, newest first, with the
English and Chinese text side by side. The website's `/en/changelog` and
`/zh/changelog` render it, and the nav links to it. When a release is published,
add its entry there and push: the site rebuilds and the entry goes live. The
entry is the release notes written for readers, not the generated changelog, so
keep it to what a user notices.

The release title carries the tag as-is (`Motty v0.1.2`, not `Motty 0.1.2`), so
the version on GitHub reads the same as in the app and in the changelog.

## The Cloudflare mirror

Publishing the draft (`gh release edit v0.1.2 --draft=false`) also triggers
`.github/workflows/mirror.yml`, which copies every asset to the `motty` R2 bucket
on Cloudflare. The bucket answers on `https://dl.motty.page`; the GitHub release
stays the source of record and the copy is only a faster path to the same bytes.

`tools/mirror-release.sh TAG` does the work and can be run locally:

```sh
tools/mirror-release.sh v0.1.2              # downloads the release with gh, then uploads
tools/mirror-release.sh v0.1.2 --dir build/release-0.1.2   # upload what is already there
```

Object layout:

| Key | Cache | What |
| --- | --- | --- |
| `releases/<tag>/<asset>` | a year, immutable | the exact release asset |
| `latest/<stable>` | five minutes | the same bytes, version out of the name (`motty-0.1.2-darwin-arm64.dmg` → `motty-darwin-arm64.dmg`) |
| `latest.json` | five minutes | version, tag, and every file's name, size and SHA-256 |

The website's download page links at `/latest/<stable>`, so it needs no rebuild
per version; `latest.json` is what a script should read. The page also lists only
what `latest.json` holds, so adding or dropping a package format with a release
shows up on the next site build without editing the page. The uploader runs
`sha256sum -c SHA256SUMS` first and refuses to mirror a release whose files do
not match it.

The workflow needs `CLOUDFLARE_API_TOKEN` with *Workers R2 Storage: Edit* (the
Pages token from docs/website.md covers the site only) and `CLOUDFLARE_ACCOUNT_ID`
(the same value the website workflow uses). Re-run a mirror from *Actions →
Mirror release → Run workflow* with the tag.

## In-app updates

The app updates itself from the mirror: it reads `https://dl.motty.page/latest.json`
(GitHub's latest release is only the fallback, and carries no checksums, so it can
send the user to the page but cannot install). The release notes in the update
dialog are the `notes` field of that file, which `tools/mirror-release.sh` copies
from the GitHub release body.

What the app downloads is chosen by name, so three things must stay as they are:

- the stable asset names in `update.StableName` (`motty-darwin-<arch>.zip`,
  `motty-linux-<arch>.tar.gz`, `motty-windows-amd64-setup.exe`) match what
  `viva package` produces, after the mirror takes the version out;
- `latest.json` keeps `version`, `tag` (`v` plus the version) and, per file, `name`,
  `size` and `sha256`;
- the program in the macOS zip is signed with the same Developer ID as the one it
  replaces, and the Windows installer with the same certificate. The app refuses an
  update whose signer differs, and a copy that is not signed at all is not updated
  in place.

Only a user-owned installation replaces itself (an app bundle in a writable folder,
the per-user Windows installer, a Linux `install.sh` prefix). A `.deb`, Program
Files or a portable program is sent to the release page.

## The package matrix

Every release carries the packages below for every platform and architecture:

| Platform | arm64 | amd64 |
| --- | --- | --- |
| macOS | `.dmg`, `.zip` | `.dmg`, `.zip` |
| Windows | `.exe` installer | `.exe` installer |
| Linux | `.deb`, `.tar.gz` | `.deb`, `.tar.gz` |

The macOS `.zip` holds the same signed and stapled bundle; it is what the
in-app updater downloads, so a release without it would leave macOS unable to
update in place.

`tools/release.sh` refuses to upload when one of those is missing, so a run with
a broken makensis stops instead of publishing a release with a hole in it. The
installers are built on Linux; makensis on Apple Silicon crashes (NSIS issue
#1333), which is why `--platforms windows` on a Mac only warns.

## What has been checked

The macOS signing, notarization and stapling were run for real, with a Developer ID
and Apple's notary service, on a throwaway app: Gatekeeper reports *Notarized
Developer ID* for both the `.dmg` and the stapled `.app`. The Windows signing was run
for real with `osslsigncode` and a self-signed certificate, including the
installer and a DigiCert timestamp, and the signed installer installed. Not run: the
workflow itself on GitHub, the import of a `.p12` for a certificate Apple trusts,
`signtool`, and a certificate from a Windows certificate authority.
