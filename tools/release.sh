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
# (--viva-ref picks another ref, such as HEAD for a checkout of the exact commit).
#
# --platforms picks what to build; by default everything this machine can: the
# macOS packages (.zip, .dmg) need Apple's tools and so a Mac, the Linux and
# Windows ones are cross-built anywhere. A release made on several machines, as
# .github/workflows/release.yml does, builds one platform on each.
#
# The Windows installer needs NSIS (makensis). makensis on Apple Silicon crashes
# (NSIS bug #1333), so there the installer is skipped with a note; build it on
# Windows or Linux and pass the file with --installer to include it.
#
# --upload TAG puts the packages on release TAG of the repository's origin; TAG
# must be v plus the version in viva.toml. A release that does not exist yet is
# created as a DRAFT pointing at the commit that was built; --publish then
# publishes it. The commit must already be pushed. SHA256SUMS is written when all
# three platforms were built in this run; otherwise run --sums TAG once they are
# all uploaded, which checksums what the release holds.
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

if [ -n "$sums_tag" ]; then
	scratch=$(mktemp -d)
	trap 'rm -rf "$scratch"' EXIT
	gh release download "$sums_tag" --dir "$scratch" --pattern 'motty*'
	(cd "$scratch" && sums motty* > SHA256SUMS && cat SHA256SUMS)
	gh release upload "$sums_tag" --clobber "$scratch/SHA256SUMS"
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
		package --target windows --arch "$arch" --format zip -o "$out"
	done
	if [ -n "$installer" ]; then
		cp "$installer" "$out/"
	elif "$work/viva-cli" package --target windows --arch amd64 --format nsis -o "$out" >"$work/nsis.log" 2>&1; then
		echo "   Windows installer built"
	else
		echo "   no Windows installer: $(tail -1 "$work/nsis.log" | cut -c1-110)"
		echo "   (build it on Windows or Linux and pass it with --installer)"
	fi
fi

rm -rf "$out/Motty.app"
if want mac && want linux && want windows; then
	(cd "$out" && rm -f SHA256SUMS && sums motty* > SHA256SUMS)
else
	echo "   no SHA256SUMS: this run builds only $platforms; run --sums TAG when every platform is uploaded"
fi
echo "== packages in $out"
ls -l "$out" | awk 'NR>1 {printf "   %10d  %s\n", $5, $9}'

[ -n "$tag" ] || exit 0

echo "== release $tag"
assets=("$out"/motty*)
[ ! -f "$out/SHA256SUMS" ] || assets+=("$out/SHA256SUMS")
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
