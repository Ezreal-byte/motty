package sshconn

import (
	"context"
	"path"
	"strings"
	"time"
)

const shellIntegrationProbe = `printf '\036motty-shell:%s\037' "$SHELL"`

func (conn *Conn) shellIntegrationCommand(token string) string {
	ctx, cancel := context.WithTimeout(conn.ctx, 2*time.Second)
	defer cancel()
	data, err := conn.RunCommand(ctx, shellIntegrationProbe, 4096)
	if err != nil {
		return ""
	}
	_, value, ok := strings.Cut(string(data), "\x1emotty-shell:")
	if !ok {
		return ""
	}
	shell, _, ok := strings.Cut(value, "\x1f")
	if !ok {
		return ""
	}
	return integrationCommand(shell, token)
}

// Only recognized Unix shells opt into the startup shim. The probe and shim
// never write commands to an already running interactive program.
func integrationCommand(shell string, tokens ...string) string {
	suffix := ""
	if len(tokens) > 0 && tokens[0] != "" {
		token := tokens[0]
		if len(token) > 128 || strings.IndexFunc(token, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_')
		}) >= 0 {
			return ""
		}
		suffix = ";motty=" + token
	}
	if !strings.HasPrefix(shell, "/") || strings.IndexFunc(shell, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return ""
	}
	name := path.Base(shell)
	if name != "bash" && name != "zsh" {
		return ""
	}
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
	script := `_motty_umask=$(umask)
umask 077
_motty_dir=$(mktemp -d "${TMPDIR:-/tmp}/motty-shell.XXXXXXXX") || { umask "$_motty_umask"; exec "$1" -l; }
trap 'rm -rf -- "$_motty_dir"' 0
trap 'exit 129' HUP
trap 'exit 143' TERM
`
	if name == "bash" {
		script += "cat > \"$_motty_dir/bashrc\" <<'MOTTY_BASH'\n" + bashIntegration + "\nMOTTY_BASH\n" +
			`umask "$_motty_umask"
MOTTY_SHELL_DIR=$_motty_dir "$1" --rcfile "$_motty_dir/bashrc" -i
`
	} else {
		script += `cat > "$_motty_dir/.zshenv" <<'MOTTY_ENV'
_motty_runtime=$ZDOTDIR
_motty_user_zdotdir=$MOTTY_USER_ZDOTDIR
_motty_user_zdotdir_set=$MOTTY_USER_ZDOTDIR_SET
unset MOTTY_USER_ZDOTDIR MOTTY_USER_ZDOTDIR_SET
ZDOTDIR=$_motty_user_zdotdir
[[ -r "$ZDOTDIR/.zshenv" ]] && source "$ZDOTDIR/.zshenv"
_motty_user_zdotdir=${ZDOTDIR-$HOME}
ZDOTDIR=$_motty_runtime
MOTTY_ENV
for _motty_file in .zprofile .zshrc .zlogin; do
cat > "$_motty_dir/$_motty_file" <<'MOTTY_START'
ZDOTDIR=$_motty_user_zdotdir
MOTTY_START
printf '[[ -r "$ZDOTDIR/%s" ]] && source "$ZDOTDIR/%s"\n' "$_motty_file" "$_motty_file" >> "$_motty_dir/$_motty_file"
if [ "$_motty_file" != .zlogin ]; then
cat >> "$_motty_dir/$_motty_file" <<'MOTTY_NEXT'
_motty_user_zdotdir=${ZDOTDIR-$HOME}
ZDOTDIR=$_motty_runtime
MOTTY_NEXT
fi
done
cat >> "$_motty_dir/.zlogin" <<'MOTTY_ZSH'
` + zshIntegration + `
if [[ -z $_motty_user_zdotdir_set && $ZDOTDIR == $HOME ]]; then unset ZDOTDIR; fi
rm -rf -- "$_motty_runtime"
unset _motty_runtime _motty_user_zdotdir _motty_user_zdotdir_set
MOTTY_ZSH
umask "$_motty_umask"
MOTTY_USER_ZDOTDIR=${ZDOTDIR-$HOME} MOTTY_USER_ZDOTDIR_SET=${ZDOTDIR+x} ZDOTDIR=$_motty_dir "$1" -l
`
	}
	script += "_motty_status=$?\nexit \"$_motty_status\"\n"
	return "sh -c " + quote(strings.ReplaceAll(script, "__MOTTY_SUFFIX__", suffix)) + " sh " + quote(shell)
}

// Encode bytes, not characters: file URI reserved characters and non-ASCII
// paths must round-trip through the terminal parser without shell evaluation.
const directoryReportFunction = `__motty_report() {
    local _motty_status=$? LC_ALL=C _motty_path= _motty_char _motty_hex _motty_i
    for ((_motty_i=0; _motty_i<${#PWD}; _motty_i++)); do
        _motty_char=${PWD:_motty_i:1}
        case $_motty_char in
            [a-zA-Z0-9/._~-]) _motty_path=$_motty_path$_motty_char ;;
            *) printf -v _motty_hex '%%%02X' "'$_motty_char"; _motty_path=$_motty_path%${_motty_hex: -2} ;;
        esac
    done
    printf '\033]133;A__MOTTY_SUFFIX__\007\033]7;file://%s%s\007' "${HOSTNAME:-${HOST:-}}" "$_motty_path"
    case $PS1 in
        *"$_motty_prompt_marker") ;;
        *) PS1=$PS1$_motty_prompt_marker ;;
    esac
    return "$_motty_status"
}`

const bashIntegration = `_motty_runtime=$MOTTY_SHELL_DIR
unset MOTTY_SHELL_DIR
[[ -r /etc/profile ]] && source /etc/profile
for _motty_profile in "$HOME/.bash_profile" "$HOME/.bash_login" "$HOME/.profile"; do
    if [[ -r $_motty_profile ]]; then source "$_motty_profile"; break; fi
done
# The login profiles only reach ~/.bashrc when they source it themselves; a
# server whose interactive setup lives there must still get it.
for _motty_bashrc in /etc/bash.bashrc "$HOME/.bashrc"; do
    if [[ -r $_motty_bashrc ]]; then source "$_motty_bashrc"; fi
done
` + directoryReportFunction + `
_motty_prompt_marker='\[\e]133;B__MOTTY_SUFFIX__\a\]'
case $(declare -p PROMPT_COMMAND 2>/dev/null) in
    'declare -a '*) PROMPT_COMMAND+=(__motty_report) ;;
    *) PROMPT_COMMAND=${PROMPT_COMMAND:+$PROMPT_COMMAND$'\n'}__motty_report ;;
esac
if (( BASH_VERSINFO[0] > 4 || (BASH_VERSINFO[0] == 4 && BASH_VERSINFO[1] >= 4) )); then
    PS0=$'\e]133;C__MOTTY_SUFFIX__\a'${PS0-}
    printf '\033]133;C__MOTTY_SUFFIX__\007'
fi
rm -rf -- "$_motty_runtime"
unset _motty_runtime _motty_profile`

const zshIntegration = directoryReportFunction + `
_motty_prompt_marker=$'%{\e]133;B__MOTTY_SUFFIX__\a%}'
typeset -ga precmd_functions
precmd_functions+=(__motty_report)
__motty_command_start() { printf '\033]133;C__MOTTY_SUFFIX__\007'; }
typeset -ga preexec_functions
preexec_functions=(__motty_command_start "${preexec_functions[@]}")
__motty_command_start`
