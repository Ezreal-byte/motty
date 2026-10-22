package term

import "testing"

func TestWorkingDirectoryReportsAreReadAndDeduplicated(t *testing.T) {
	terminal := New(80, 24, 0)
	var seen []Cwd
	terminal.SetCwdChange(func(cwd Cwd) { seen = append(seen, cwd) })
	terminal.Write([]byte("\x1b]7;file://box/home/me/My%20Dir\x07"))
	terminal.Write([]byte("\x1b]7;file://box/home/me/My%20Dir\x1b\\")) // the same again, ended the other way
	terminal.Write([]byte("\x1b]7;file:///var/log\x07"))
	if len(seen) != 2 || seen[0] != (Cwd{Host: "box", Path: "/home/me/My Dir"}) || seen[1] != (Cwd{Path: "/var/log"}) {
		t.Fatalf("reports = %+v", seen)
	}
	if cwd, ok := terminal.WorkingDirectory(); !ok || cwd.Path != "/var/log" {
		t.Fatalf("WorkingDirectory = %+v, %v", cwd, ok)
	}
}

func TestWorkingDirectoryReportsAcrossEverySplit(t *testing.T) {
	stream := []byte("x\x1b]7;file://h/a/b%C3%A9\x07y")
	for cut := 1; cut < len(stream); cut++ {
		terminal := New(80, 24, 0)
		terminal.Write(stream[:cut])
		terminal.Write(stream[cut:])
		if cwd, ok := terminal.WorkingDirectory(); !ok || cwd.Path != "/a/bé" {
			t.Fatalf("cut at %d: %+v, %v", cut, cwd, ok)
		}
	}
}

func TestMalformedWorkingDirectoriesAreRefused(t *testing.T) {
	for _, report := range []string{
		"relative/path", "file://host", "file://host/bad%00path", "http://host/path", "file://host/%0a", "file://host/..%0d",
	} {
		terminal := New(80, 24, 0)
		terminal.Write([]byte("\x1b]7;" + report + "\x07"))
		if cwd, ok := terminal.WorkingDirectory(); ok {
			t.Errorf("%q was accepted as %+v", report, cwd)
		}
	}
}
