#!/usr/bin/env bash
# Rebuild and restart motty whenever a .go file in this repository or in ../viva
# changes. Opens a real window on every restart; needs fswatch (brew install fswatch).
set -u
cd "$(dirname "$0")/.."

bin=/tmp/motty-dev
pid=

# A rebuilt ad-hoc binary is a new code identity, so macOS asks for the vault key
# on every restart. Sign the dev binary with a stable Developer ID instead; the
# designated requirement then stays the same across builds. Set
# MOTTY_DEV_SIGN_IDENTITY to override, or to "none" to build unsigned.
sign_identity=${MOTTY_DEV_SIGN_IDENTITY:-}
if [ "$sign_identity" = none ]; then
	sign_identity=
elif [ -z "$sign_identity" ]; then
	sign_identity=$(security find-identity -v -p codesigning 2>/dev/null | sed -n 's/.*"\(.*\)"/\1/p' | head -1)
fi

# Colour only a terminal: a redirected log or NO_COLOR stays plain.
use_color=0
if [ -t 1 ] && [ -z "${NO_COLOR+x}" ] && [ "${TERM:-}" != dumb ]; then
	use_color=1
fi

say() {
	local tone=$1
	shift
	local mark='==>' text="$*"
	if [ "$use_color" -eq 1 ]; then
		local prefix='' reset=$'\033[0m'
		case "$tone" in
		building) prefix=$'\033[1;34m' ;; # blue
		ok) prefix=$'\033[1;32m' ;;       # green
		wait) prefix=$'\033[1;33m' ;;     # yellow
		change) prefix=$'\033[1;36m' ;;   # cyan
		fail) prefix=$'\033[1;31m' ;;     # red
		dim) prefix=$'\033[2m' ;;         # dim
		esac
		mark="${prefix}==>${reset}"
		text="${prefix}${text}${reset}"
	fi
	printf '%s %s\n' "$mark" "$text"
}

stop() {
	if [ -n "$pid" ]; then
		say wait "stopping motty-dev (pid $pid)"
		kill "$pid" 2>/dev/null
		wait "$pid" 2>/dev/null
		pid=
	fi
}

start() {
	say building "building motty-dev from $PWD"
	SECONDS=0
	if go build -o "$bin" ./cmd/motty; then
		say ok "build finished in ${SECONDS}s"
		if [ -n "$sign_identity" ]; then
			if output=$(codesign --force --identifier dev.motty.app --timestamp=none --sign "$sign_identity" "$bin" 2>&1); then
				say dim "signed with $sign_identity"
			else
				say fail "codesign failed; running unsigned: $output"
			fi
		fi
		"$bin" &
		pid=$!
		if [ -n "${MOTTY_DATA_DIR:-}" ]; then
			say ok "started motty-dev pid $pid (data: $MOTTY_DATA_DIR)"
		else
			say ok "started motty-dev pid $pid (data: default)"
		fi
	else
		say fail "build failed; waiting for the next change" >&2
	fi
}

trap 'stop; exit 0' INT TERM

start
say dim "watching . and ../viva for Go changes (Ctrl-C to stop)"
while read -r _; do
	say change "change detected; restarting"
	stop
	start
done < <(fswatch -o -l 0.3 -e '.*' -i '\.go$' -i 'go\.(mod|sum)$' . ../viva)
