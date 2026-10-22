package capture

import (
	"bytes"
	"math/rand"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

// runShell emulates a pty without a pty: the typed bytes are echoed (with
// onlcr) first, then the real /bin/sh output follows.
func runShell(t *testing.T, command string) (Result, string) {
	t.Helper()
	nonce := NewNonce()
	sent := Wrap(command, nonce)
	cmd := exec.Command("sh")
	cmd.Stdin = bytes.NewReader(sent)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("sh: %v\n%s", err, out.String())
	}
	stream := strings.ReplaceAll(string(sent), "\n", "\r\n") + strings.ReplaceAll(out.String(), "\n", "\r\n") + "$ "
	tp := NewTap(nonce, 4096)
	tp.SetEcho(sent)
	rng := rand.New(rand.NewSource(1))
	var disp []byte
	done := false
	for b := []byte(stream); len(b) > 0; {
		n := min(1+rng.Intn(11), len(b))
		d, dn := tp.Feed(b[:n])
		disp = append(disp, d...)
		done = done || dn
		b = b[n:]
	}
	if !done {
		t.Fatalf("not done; stream %q", stream)
	}
	if strings.Contains(string(disp), markerPrefix+nonce) {
		t.Fatalf("marker visible: %q", disp)
	}
	return tp.Result(), string(disp)
}

func TestShellIntegration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs sh")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not found")
	}
	tests := []struct {
		name, cmd, output string
		code              int
	}{
		{"echo", "echo hi", "hi", 0},
		{"false", "false", "", 1},
		{"subshell exit", "(exit 7)", "", 7},
		{"multi-line", "echo a\necho b\nfalse", "a\nb", 1},
		{"trailing comment", "echo hi # a comment", "hi", 0},
		{"comment only line", "echo x\n# done", "x", 0},
		{"and-or", "true && echo yes || echo no", "yes", 0},
		{"or", "false || echo fallback", "fallback", 0},
		{"and fails", "false && echo no", "", 1},
		{"heredoc", "cat <<EOF\nline1\nline2\nEOF", "line1\nline2", 0},
		{"trailing backslash", "echo hi \\", "hi", 0},
		{"no trailing newline output", "printf abc", "abc", 0},
		{"pipeline", "printf 'b\\na\\n' | sort", "a\nb", 0},
		{"exit 3 in group status", "sh -c 'exit 3'", "", 3},
		{"stderr", "echo err >&2", "err", 0},
		{"empty", "", "", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := runShell(t, tc.cmd)
			if r.Output != tc.output || r.ExitCode != tc.code || !r.Done || r.Truncated {
				t.Errorf("got %+v; want output %q code %d", r, tc.output, tc.code)
			}
		})
	}
}

func TestShellUnterminatedQuoteNeverCompletes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs sh")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not found")
	}
	nonce := NewNonce()
	sent := Wrap("echo 'oops", nonce)
	cmd := exec.Command("sh")
	cmd.Stdin = bytes.NewReader(sent)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	_ = cmd.Run() // syntax error at EOF: no marker is printed
	tp := NewTap(nonce, 1000)
	if _, done := tp.Feed(append([]byte(strings.ReplaceAll(string(sent), "\n", "\r\n")), out.Bytes()...)); done {
		t.Fatal("must not complete")
	}
}
