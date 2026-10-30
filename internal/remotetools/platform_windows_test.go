package remotetools

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

type windowsCommandRunner struct{}

func (windowsCommandRunner) RunCommand(ctx context.Context, c string, _ int) ([]byte, error) {
	args := strings.Fields(c)
	return exec.CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
}
func TestWindowsRemoteProcessScriptOnRealCIM(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	rows, err := ReadSafeProcessSnapshot(ctx, ForSystem(windowsCommandRunner{}, "windows"), SortCPU)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows.Processes) == 0 {
		t.Fatal("no real processes")
	}
	for _, p := range rows.Processes {
		if p.PID <= 0 || p.Command == "" || p.Elapsed == "" || p.StartIdentity != "" {
			t.Fatalf("invalid process %+v", p)
		}
	}
}
