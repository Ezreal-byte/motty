#!/bin/bash
# Build the release packages of Motty from a clean checkout, checksum them, and
# optionally upload them to a GitHub release.
#
#   tools/release.sh [--ref REF] [--out DIR] [--platforms mac,linux,windows]
#                    [--installer FILE] [--viva-ref REF]
#                    [--upload TAG [--notes FILE] [--publish]]
#   tools/release.sh --sums TAG
#
# The packages come from REF (default HEAD), built in a temporary worktree so
# uncommitted changes never leak in, with the sibling ../viva at its main branch
# (--viva-ref picks another ref; the release workflow passes the exact commit it
# resolved once, so every package of a release uses the same frame).
#
# --platforms picks what to build; by default everything this machine can: the
# macOS packages (.dmg and .zip) need Apple's tools and so a Mac, the Windows
# ones are cross-built anywhere. A release made on several machines, as
# .github/workflows/release.yml does, builds one platform on each.
#
# The Windows packages are NSIS installers, one per architecture, and NSIS
# (makensis) must be able to run. The arm64 installer needs makensis 3.10 or
# newer; if makensis is missing or crashes, that architecture is skipped with a
# note, and a prebuilt amd64 installer can be passed with --installer.
#
# --upload TAG puts the packages on release TAG of the repository's origin; TAG
# must be v plus the version in viva.toml, and the run refuses to upload unless
# every package of the platforms it built is there. A release that does not exist yet is
# created as a DRAFT pointing at the commit that was built; --publish then
# publishes it. The commit must already be pushed. SHA256SUMS is written when all
# three platforms were built in this run; otherwise run --sums TAG once they are
# all uploaded, which signs and checksums what the release holds.
#
# The packages of a release are signed with the update key in
# MOTTY_UPDATE_SIGNING_KEY (the minisign private key itself, not a path), and the
# signatures are uploaded as SIGNATURES. The signatures for a whole release are
# written by --sums, not by the jobs that build it: a job sees only its own
# platform's packages, so a manifest it uploaded would be replaced by the next
# job's and leave that platform without a signature. Finalizing without the key
# is refused: a release nobody can update in place is a release with a broken
# update path.
#
# Signing is optional and comes from the environment, which viva package reads:
# with APPLE_SIGNING_IDENTITY (or APPLE_CERTIFICATE and APPLE_CERTIFICATE_PASSWORD)
# the macOS bundles and images are signed, and with notarytool credentials as well
# (APPLE_NOTARY_PROFILE, or APPLE_API_KEY/_KEY_ID/_ISSUER, or APPLE_ID/
# APPLE_PASSWORD/APPLE_TEAM_ID) the .dmg is notarized and stapled; with
# WINDOWS_CERTIFICATE_FILE (or WINDOWS_CERTIFICATE) and WINDOWS_CERTIFICATE_PASSWORD,
# or WINDOWS_SIGN_COMMAND, the Windows programs and the installer are signed. See
# `viva help package` and docs/release.md. Without them nothing is signed. The
# Linux packages are never signed.
set -euo pipefail

here=$(cd "$(dirname "$0")/.." && pwd)
ref=HEAD viva_ref=main out="" installer="" tag="" notes="" publish=0 sums_tag="" platforms=""
while [ $# -gt 0 ]; do
	case "$1" in
	--ref) ref=$2; shift 2 ;;
	--viva-ref) viva_ref=$2; shift 2 ;;
	--out) out=$2; shift 2 ;;
	--platforms) platforms=$2; shift 2 ;;
	--installer) installer=$2; shift 2 ;;
	--upload) tag=$2; shift 2 ;;
	--notes) notes=$2; shift 2 ;;
	--publish) publish=1; shift ;;
	--sums) sums_tag=$2; shift 2 ;;
	-h|--help) sed -n '2,/^set -euo/p' "$0" | sed '$d' | sed 's/^# \{0,1\}//'; exit 0 ;;
	*) echo "unknown argument: $1" >&2; exit 2 ;;
	esac
done

sums() { if command -v sha256sum >/dev/null 2>&1; then sha256sum "$@"; else shasum -a 256 "$@"; fi; }
sums_packages() { if [ -f SIGNATURES ]; then sums motty* SIGNATURES; else sums motty*; fi; }

# A run that is going to upload must produce the whole matrix: a missing package
# would otherwise be published as a release nobody can install from.
strict=0
[ -z "$tag" ] || strict=1

if [ -n "$sums_tag" ]; then
	version=$(sed -n 's/^version *= *"\([^"]*\)".*/\1/p' "$here/viva.toml" | head -1)
	[ -n "$version" ] || { echo "no version in viva.toml" >&2; exit 1; }
	scratch=$(mktemp -d)
	trap 'rm -rf "$scratch"' EXIT
	gh release download "$sums_tag" --dir "$scratch" --pattern 'motty*'
	# Sign here, once, with the whole matrix in hand: a build job only sees the
	# packages of its own platform, so a manifest written there is overwritten by
	# the next job's upload and leaves that platform out of the release.
	[ -n "${MOTTY_UPDATE_SIGNING_KEY:-}" ] || {
		echo "MOTTY_UPDATE_SIGNING_KEY is not set: refusing to finalize a release that cannot be updated in place" >&2
		exit 1
	}
	key=$scratch/update.key
	(umask 077 && printf '%s\n' "$MOTTY_UPDATE_SIGNING_KEY" >"$key")
	(cd "$here" && go run ./tools/updatesign sign -key "$key" -version "$version" -dir "$scratch")
	(cd "$here" && go run ./tools/updatesign verify -version "$version" -dir "$scratch")
	(cd "$scratch" && sums_packages > SHA256SUMS && cat SHA256SUMS)
	gh release upload "$sums_tag" --clobber "$scratch/SIGNATURES" "$scratch/SHA256SUMS"
	exit 0
fi

if [ -z "$platforms" ]; then
	if [ "$(uname -s)" = Darwin ]; then platforms=mac,linux,windows; else platforms=linux,windows; fi
fi
want() { case ",$platforms," in *",$1,"*) return 0 ;; *) return 1 ;; esac; }
for platform in ${platforms//,/ }; do
	case "$platform" in
	mac|linux|windows) ;;
	*) echo "unknown platform: $platform (mac, linux, windows)" >&2; exit 2 ;;
	esac
done
if want mac && [ "$(uname -s)" != Darwin ]; then
	echo "the macOS packages are built on macOS; this is $(uname -s). Use --platforms linux,windows here" >&2
	exit 1
fi
viva_repo=$(cd "$here/../viva" 2>/dev/null && pwd) || { echo "the Viva checkout is expected at ../viva" >&2; exit 1; }

commit=$(git -C "$here" rev-parse --verify "$ref^{commit}")
short=$(git -C "$here" rev-parse --short "$commit")
version=$(sed -n 's/^version *= *"\([^"]*\)".*/\1/p' "$here/viva.toml" | head -1)
[ -n "$version" ] || { echo "no version in viva.toml" >&2; exit 1; }
if [ -n "$tag" ] && [ "$tag" != "v$version" ]; then
	echo "the release tag is $tag but viva.toml says version $version: the tag must be v$version" >&2
	exit 1
fi
out=${out:-$here/build/release-$version}
mkdir -p "$out"
out=$(cd "$out" && pwd)
[ -z "$installer" ] || [ -f "$installer" ] || { echo "no such installer: $installer" >&2; exit 1; }

work=$(mktemp -d)
cleanup() {
	git -C "$here" worktree remove --force "$work/motty" 2>/dev/null || true
	git -C "$viva_repo" worktree remove --force "$work/viva" 2>/dev/null || true
	rm -rf "$work"
}
trap cleanup EXIT

echo "== Motty $version from $short ($platforms)"
git -C "$viva_repo" worktree add -q --detach "$work/viva" "$viva_ref"
git -C "$here" worktree add -q --detach "$work/motty" "$commit"
(cd "$work/viva" && go build -o "$work/viva-cli" ./cmd/viva)

cd "$work/motty"
package() { "$work/viva-cli" package "$@" 2>&1 | sed 's/^/   /'; }

if want mac; then
	echo "== macOS"
	for arch in arm64 amd64; do
		package --arch "$arch" -o "$work/mac-$arch/Motty.app" --format zip,dmg
		cp "$work/mac-$arch/"motty-*.zip "$work/mac-$arch/"motty-*.dmg "$out/"
	done
fi
if want linux; then
	echo "== Linux"
	for arch in amd64 arm64; do
		package --target linux --arch "$arch" --format tar.gz,deb -o "$out"
	done
fi
if want windows; then
	echo "== Windows"
	for arch in amd64 arm64; do
		if [ -n "$installer" ] && [ "$arch" = amd64 ]; then
			cp "$installer" "$out/"
			echo "   Windows amd64 installer copied from $installer"
		elif "$work/viva-cli" package --target windows --arch "$arch" --format nsis -o "$out" >"$work/nsis-$arch.log" 2>&1; then
			echo "   Windows $arch installer built"
		else
			echo "   no Windows $arch installer: $(tail -1 "$work/nsis-$arch.log" | cut -c1-110)"
			echo "   (makensis needs to work; a prebuilt amd64 installer can be passed with --installer)"
		fi
	done
fi

rm -rf "$out/Motty.app"

if [ "$strict" = 1 ]; then
	missing=0
	have() { compgen -G "$out/$1" >/dev/null || { echo "   missing package: $1" >&2; missing=1; }; }
	if want mac; then
		have 'motty-*-darwin-arm64.dmg'
		have 'motty-*-darwin-amd64.dmg'
		# The zip is what the in-app updater downloads (update.StableName).
		have 'motty-*-darwin-arm64.zip'
		have 'motty-*-darwin-amd64.zip'
	fi
	if want linux; then
		have 'motty-*-linux-arm64.tar.gz'
		have 'motty-*-linux-amd64.tar.gz'
		have 'motty_*_arm64.deb'
		have 'motty_*_amd64.deb'
	fi
	if want windows; then
		have 'motty-*-windows-amd64-setup.exe'
		have 'motty-*-windows-arm64-setup.exe'
	fi
	[ "$missing" = 0 ] || { echo "refusing to upload an incomplete release" >&2; exit 1; }
fi

# The update signature is what lets a Windows build update itself without a
# code-signing certificate, and what binds a package to its version everywhere.
signing_key=
if [ -n "${MOTTY_UPDATE_SIGNING_KEY:-}" ]; then
	signing_key=$work/update.key
	(umask 077 && printf '%s\n' "$MOTTY_UPDATE_SIGNING_KEY" >"$signing_key")
elif [ -n "$tag" ]; then
	echo "MOTTY_UPDATE_SIGNING_KEY is not set: refusing to upload a release that cannot be updated in place" >&2
	exit 1
else
	echo "warning: MOTTY_UPDATE_SIGNING_KEY is not set; the packages will not be installable in place" >&2
fi
if [ -n "$signing_key" ]; then
	echo "== signing packages"
	(cd "$work/motty" && go run ./tools/updatesign sign -key "$signing_key" -version "$version" -dir "$out")
	(cd "$work/motty" && go run ./tools/updatesign verify -version "$version" -dir "$out")
fi

if want mac && want linux && want windows; then
	(cd "$out" && rm -f SHA256SUMS && sums_packages > SHA256SUMS)
else
	echo "   no SHA256SUMS: this run builds only $platforms; run --sums TAG when every platform is uploaded"
fi
echo "== packages in $out"
ls -l "$out" | awk 'NR>1 {printf "   %10d  %s\n", $5, $9}'

[ -n "$tag" ] || exit 0

echo "== release $tag"
assets=("$out"/motty*)
# Only a run that built every platform in one place uploads a manifest. The
# finalize step signs the whole matrix into one SIGNATURES; a per-platform one
# would be overwritten by the other build job and leave its packages unsigned.
if [ -f "$out/SHA256SUMS" ]; then
	assets+=("$out/SHA256SUMS")
	[ ! -f "$out/SIGNATURES" ] || assets+=("$out/SIGNATURES")
fi
if gh release view "$tag" >/dev/null 2>&1; then
	gh release upload "$tag" --clobber "${assets[@]}"
else
	if ! git -C "$here" branch -r --contains "$commit" | grep -q .; then
		echo "commit $short is not on any remote branch; push it first" >&2
		exit 1
	fi
	args=(--draft --title "Motty $version" --target "$commit")
	[ -z "$notes" ] || args+=(--notes-file "$notes")
	[ -n "$notes" ] || args+=(--generate-notes)
	gh release create "$tag" "${args[@]}" "${assets[@]}"
fi
if [ "$publish" = 1 ]; then
	gh release edit "$tag" --draft=false
else
	echo "   still a draft; publish with: gh release edit $tag --draft=false"
fi
