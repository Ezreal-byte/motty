package term

import (
	"net/url"
	"strings"
)

// The shell's working directory, as OSC 7 reports it: "file://host/path", sent by
// a prompt that has been set up to. The directory is only ever data for the
// owner to look at; nothing here touches a file system.

// maxCwd bounds the path kept.
const maxCwd = 2048

// Cwd is a directory the shell reported.
type Cwd struct {
	// Host is the machine name the shell gave, possibly empty.
	Host string
	// Path is the directory, absolute and cleaned of control characters.
	Path string
}

// SetCwdChange installs the receiver of directory reports. The callback runs
// during Write, only when the directory differs from the last one reported, and
// must not block.
func (t *Terminal) SetCwdChange(fn func(Cwd)) { t.cwdFn = fn }

// WorkingDirectory is the last directory the shell reported, or false.
func (t *Terminal) WorkingDirectory() (Cwd, bool) { return t.cwd, t.cwd.Path != "" }

// osc7 reads a working-directory report.
func (t *Terminal) osc7(text []byte) {
	cwd, ok := parseCwd(string(text))
	if !ok || cwd == t.cwd {
		return
	}
	t.cwd = cwd
	if t.cwdFn != nil {
		t.cwdFn(cwd)
	}
}

// parseCwd reads "file://host/percent-encoded/path". Anything else, a relative
// path or a path with control characters, is refused.
func parseCwd(value string) (Cwd, bool) {
	if len(value) > maxCwd*3 || !strings.HasPrefix(value, "file://") {
		return Cwd{}, false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Path == "" || parsed.Path[0] != '/' || len(parsed.Path) > maxCwd {
		return Cwd{}, false
	}
	for _, r := range parsed.Path {
		if r < 0x20 || r == 0x7f {
			return Cwd{}, false
		}
	}
	return Cwd{Host: parsed.Hostname(), Path: parsed.Path}, true
}
