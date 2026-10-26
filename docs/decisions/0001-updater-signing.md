# 0001 — Sign releases with a key the app carries

Status: accepted, 2026-10-26.

## Context

Motty installs its own updates from the R2 mirror. Until 0.1.1 the only proof
that a package came from us was the code signature the operating system checks:
the Developer ID team on macOS and the Authenticode subject on Windows. That has
two problems.

- The Windows packages are unsigned, so a Windows install can never replace
  itself; the update dialog downloads nothing and sends the user to the release
  page. Fixing it by buying a code-signing certificate is a recurring cost and
  still leaves the second problem.
- The feed itself is not signed. A compromised mirror can point at an older
  package that we really did sign and call it a newer version. The OS signature
  still checks out, so the app would install it.

Tauri solves both the same way: it embeds an Ed25519 (minisign) public key in the
application and verifies a signature that travels with the update. The operating
system signature stays a separate, optional layer.

## Decision

- Every package of a release is signed with a dedicated minisign key. The
  public half is compiled into the app (`internal/updatesign`); the private half
  lives in the `MOTTY_UPDATE_SIGNING_KEY` release secret and in the owner's
  offline backup, never in this repository.
- `tools/release.sh` signs the packages and uploads a `SIGNATURES` asset: one
  `name<TAB>base64` line per package. `tools/mirror-release.sh` copies the
  matching signature into `latest.json`, per file.
- The signature's trusted comment names the version and the file, and both are
  signed together with the package. Verification checks the package bytes, then
  the comment against the version the feed announced and the file the app
  resolved. A feed that is changed can therefore neither replay an older package
  as a newer version nor serve a package for another platform.
- `selfupdate` refuses to replace the program unless the signature verifies. On
  macOS the Developer ID team check stays. On Windows, if both the running copy
  and the installer are signed, their subjects must still match; an unsigned
  installer is accepted on the strength of the update signature alone.
- The update dialog only offers an in-place install when the release carries a
  package *and* a signature. Otherwise it says why and links to the release
  page.
- `tools/release.sh --upload` refuses a release without the signing key. A
  release nobody can update in place is a broken release.

## Consequences

- Windows updates work without a code-signing certificate; SmartScreen still
  warns on first install.
- The signing key is the trust root for every 0.1.2-or-later installation. If it
  is lost, those installations cannot be updated in place again; if it leaks,
  its holder can make them run arbitrary code until a build with a new key
  reaches them. It is offline material, not a CI-only convenience.
- Rotation is a migration, not a config change: the app embeds one key, so the
  release that introduces the new key has to be signed with the old one, and a
  user who skips that release cannot follow. A future change can carry a list of
  keys and overlap the two, at the cost of a second signature field in the feed.
- The mirror can withhold an update, or serve a package that is correctly signed
  for an older version; it cannot make the app install one as a newer version.
- The GitHub release stays the source of record: `SIGNATURES` is published
  there, so a package can be checked without the mirror.

## Migration

- 0.1.1 and older ignore the `signature` field and keep their old behaviour.
- 0.1.2 is the first signed release. 0.1.1 on macOS updates to it in place via
  the Developer ID check as before. 0.1.1 on Windows cannot install anything
  itself, so it reaches 0.1.2 as a manual download; from 0.1.2 on, Windows
  updates in place.
- Releases after 0.1.2 must keep `SIGNATURES` and the key. Re-running the mirror
  for an old, unsigned release leaves its feed entries unsigned.

## Alternatives considered

- **Buy a Windows code-signing certificate.** Fixes SmartScreen and keeps the
  operating-system anchor, but costs money, involves certificate lifetimes and
  still does not bind the feed to a version.
- **electron-updater's model (compare the Authenticode publisher).** Works only
  when both copies are signed, so it has the same dead end for our unsigned
  Windows builds.
- **A custom Ed25519 envelope over `tag + file + sha256 + size`.** No new
  dependency, but a format only this project can verify. minisign is a small,
  well-reviewed format with an independent CLI, and the signed comment already
  gives us the version binding.
