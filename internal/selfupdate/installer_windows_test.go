package selfupdate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// The installer fixture records the actual CreateProcess command line. It
// never installs anything or opens Motty, including for a real-looking /D= path.
func TestMain(m *testing.M) {
	if output := os.Getenv("MOTTY_TEST_INSTALLER_COMMAND_LINE"); output != "" {
		line := windows.UTF16PtrToString(windows.GetCommandLine())
		if err := os.WriteFile(output, []byte(line), 0600); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestWindowsInstallerKeepsTheExactCustomDirectory(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	installer := filepath.Join(t.TempDir(), "Motty 更新 setup.exe")
	if err := copyExecutable(exe, installer); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{
		`D:\Program Files\motty`,
		`D:\开发 工具\Motty & SSH`,
		`C:\Users\me\AppData\Local\Programs\Motty`,
	} {
		t.Run(root, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "command-line.txt")
			t.Setenv("MOTTY_TEST_INSTALLER_COMMAND_LINE", output)
			if err := (Job{Staged: installer, Root: root}).applyWindows(); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			_, dir, found := strings.Cut(string(data), " /S /D=")
			if !found || dir != root {
				t.Fatalf("NSIS would install elsewhere: %q, want unquoted /D=%s at the end", data, root)
			}
		})
	}
}
