// Package capture runs one command inside the user's visible interactive
// terminal and learns how it ended.
//
// The command is typed into the pty wrapped by [Wrap]; a [Tap] watches the pty
// output, removes the completion marker from what the user sees, and records
// the command's plain-text output and exit status.
//
// The wrapper targets POSIX-style interactive shells (sh, bash, zsh, dash,
// ksh). Shells without "{ ...; }" groups and "$?" (fish, nushell, cmd.exe,
// PowerShell) never print the marker, so the tap never completes and the caller
// must time out.
package capture

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

const markerPrefix = "GOPAI_"

// NewNonce returns a fresh random nonce (16 hex digits) for one command.
func NewNonce() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "0000000000000000"
	}
	return hex.EncodeToString(b[:])
}

// cleanNonce keeps only [A-Za-z0-9]; both Wrap and NewTap apply it so that they
// always agree, and so that the nonce is safe to place unquoted in a shell word.
func cleanNonce(n string) string {
	var sb strings.Builder
	for i := 0; i < len(n); i++ {
		c := n[i]
		if c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			sb.WriteByte(c)
		}
	}
	if sb.Len() == 0 {
		return "0"
	}
	return sb.String()
}

// Wrap returns the exact bytes to type into the pty for one command. It ends
// with a newline and has the form
//
//	{ <command>
//	}; printf '\n%s%s%d\n' GOPAI_ <nonce> $?
//
// Design notes:
//   - The command is one brace group, so "$?" is the status of the whole
//     command (lists, pipelines, subshells), and a command that ends with a
//     comment cannot swallow the printf: the closing brace sits on its own line.
//   - Heredocs work because their terminator is followed by a newline before
//     the closing brace.
//   - The marker is printed from split pieces. The echoed command line only
//     contains "GOPAI_ <nonce> $?", never the contiguous "GOPAI_<nonce>", so a
//     terminal echo cannot be mistaken for the real marker.
//   - Only newline and tab survive as control characters: CR and CRLF become a
//     newline, every other C0 control and DEL is dropped, so the text can never
//     act as line editing or as a signal.
//   - One trailing backslash (a pending line continuation, meaningless at the
//     end of input) is dropped; otherwise it would glue the closing brace onto
//     the command line.
//   - An unterminated quote, an unfinished heredoc or an unfinished construct
//     cannot be repaired: the shell keeps waiting for input and the marker is
//     never printed, the Tap never completes, and the caller times out, calls
//     [Tap.Abort] and interrupts the shell (Ctrl-C) itself.
//   - An empty command becomes ":" because an empty group is a syntax error.
func Wrap(command, nonce string) []byte {
	cmd := sanitizeCommand(command)
	var sb strings.Builder
	sb.Grow(len(cmd) + 64)
	sb.WriteString("{ ")
	sb.WriteString(cmd)
	sb.WriteString("\n}; printf '\\n%s%s%d\\n' ")
	sb.WriteString(markerPrefix)
	sb.WriteByte(' ')
	sb.WriteString(cleanNonce(nonce))
	sb.WriteString(" $?\n")
	return []byte(sb.String())
}

// DisplayCommand returns the command text Wrap puts inside its group. The
// terminal shows this instead of the wrapper, so the text is cleaned exactly as
// the shell receives it.
func DisplayCommand(command string) string {
	return sanitizeCommand(command)
}

func sanitizeCommand(command string) string {
	b := make([]byte, 0, len(command))
	for i := 0; i < len(command); i++ {
		c := command[i]
		switch {
		case c == '\r':
			if i+1 < len(command) && command[i+1] == '\n' {
				continue
			}
			b = append(b, '\n')
		case c == '\n' || c == '\t' || c >= 0x20 && c != 0x7f:
			b = append(b, c)
		}
	}
	s := strings.TrimRight(string(b), "\n \t")
	// Drop one pending line continuation (odd number of trailing backslashes).
	n := 0
	for n < len(s) && s[len(s)-1-n] == '\\' {
		n++
	}
	if n%2 == 1 {
		s = strings.TrimRight(s[:len(s)-1], " \t")
	}
	if strings.TrimSpace(s) == "" {
		return ":"
	}
	return s
}
