package selfupdate

import (
	"os/exec"
	"syscall"
)

func installerCommandLine(cmd *exec.Cmd, dir string) {
	// NSIS consumes /D= and the rest of the command line as the directory.
	// It must be last and unquoted, even when the path contains spaces:
	// https://nsis.sourceforge.io/Docs/Chapter3.html#installerusage
	// Go's usual argument quoting would quote the entire /D= argument.
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CmdLine = syscall.EscapeArg(cmd.Path) + " /S /D=" + dir
}
