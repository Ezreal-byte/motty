package selfupdate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"motty/internal/update"
)

// Phase is what Prepare is doing, for the progress it shows.
type Phase uint8

const (
	PhaseDownloading Phase = iota
	PhaseVerifying
)

// ErrUnsupported means this installation cannot replace itself, or the release
// has no package for it.
var ErrUnsupported = errors.New("selfupdate: this installation cannot be updated in place")

// Staged is a downloaded and checked version, ready to replace the running one.
type Staged struct {
	Install Install
	Version string
	// Path is the new .app, the installer, or the directory of the unpacked
	// archive, depending on the installation's Kind.
	Path string
}

// Updater replaces the program it runs in.
type Updater struct {
	// Dir is where downloads and the helper are kept between runs.
	Dir    string
	Client *http.Client

	install Install
	reason  Reason
}

// New describes the running program and keeps updates in dir.
func New(dir string) *Updater {
	updater := &Updater{Dir: dir}
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		updater.reason = ReasonNotUserInstall
		return updater
	}
	updater.install, updater.reason = Detect(exe, SystemEnv())
	return updater
}

// Support says whether this installation can be updated in place, and if not why.
func (updater *Updater) Support() Reason { return updater.reason }

// TakeFailure is why the last update failed, if it did.
func (updater *Updater) TakeFailure() string { return TakeFailure(updater.Dir) }

// clean removes what an earlier update left, except the record of its failure.
func (updater *Updater) clean() {
	entries, err := os.ReadDir(updater.Dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.Name() != failureFile {
			os.RemoveAll(filepath.Join(updater.Dir, entry.Name()))
		}
	}
}

// Prepare downloads the release's package and checks it against the running
// program: its checksum, and where there is a signature, who signed it. Nothing
// is changed on the system; a failure or a cancelled context leaves it as it was.
func (updater *Updater) Prepare(ctx context.Context, release update.Release, progress func(Progress), phase func(Phase)) (Staged, error) {
	if updater.reason != ReasonNone || !release.Asset.Has() {
		return Staged{}, ErrUnsupported
	}
	install := updater.install
	// Who signed the running program decides who may sign its replacement, and
	// a program with no such signature cannot be updated safely: ask before
	// downloading anything.
	var want Identity
	var err error
	switch install.Kind {
	case KindMacApp:
		want, err = macIdentity(ctx, install.Root)
	case KindWindowsSetup:
		want, err = windowsIdentity(ctx, install.Exe)
	}
	if err != nil {
		return Staged{}, err
	}

	updater.clean()
	dir := filepath.Join(updater.Dir, release.Version)
	if phase != nil {
		phase(PhaseDownloading)
	}
	archive, err := Download(ctx, updater.Client, release.Asset, dir, progress)
	if err != nil {
		return Staged{}, err
	}
	if phase != nil {
		phase(PhaseVerifying)
	}
	staged := Staged{Install: install, Version: release.Version}
	switch install.Kind {
	case KindMacApp:
		unpacked := filepath.Join(dir, "unpacked")
		if err := ExtractZip(archive, unpacked); err != nil {
			return Staged{}, err
		}
		app := filepath.Join(unpacked, filepath.Base(install.Root))
		if info, err := os.Stat(app); err != nil || !info.IsDir() {
			return Staged{}, fmt.Errorf("selfupdate: the package does not hold %s", filepath.Base(install.Root))
		}
		if err := verifyMac(ctx, app, want); err != nil {
			return Staged{}, err
		}
		staged.Path = app
	case KindWindowsSetup:
		if err := verifyWindows(ctx, archive, want); err != nil {
			return Staged{}, err
		}
		staged.Path = archive
	case KindLinuxPrefix:
		unpacked := filepath.Join(dir, "unpacked")
		if err := ExtractTarGz(archive, unpacked); err != nil {
			return Staged{}, err
		}
		script, err := findInstallScript(unpacked)
		if err != nil {
			return Staged{}, err
		}
		if _, err := os.Stat(filepath.Join(script, filepath.Base(install.Exe))); err != nil {
			return Staged{}, fmt.Errorf("selfupdate: the package does not hold %s", filepath.Base(install.Exe))
		}
		staged.Path = script
	}
	return staged, nil
}

// findInstallScript is the directory of the unpacked archive that holds
// install.sh: the archive's one top-level directory.
func findInstallScript(root string) (string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			if _, err := os.Stat(filepath.Join(root, entry.Name(), "install.sh")); err == nil {
				return filepath.Join(root, entry.Name()), nil
			}
		}
	}
	return "", errors.New("selfupdate: the package holds no install.sh")
}

// Launch starts the helper that will replace the program once this process has
// quit and start the new version. The caller must quit right after it returns
// nil; the helper waits for that. The helper runs from a copy of the program, so
// it holds no file of the installation that is being replaced.
func (updater *Updater) Launch(staged Staged) error {
	exe := staged.Install.Exe
	helperDir := filepath.Join(updater.Dir, "helper")
	if err := os.MkdirAll(helperDir, 0o700); err != nil {
		return err
	}
	helper := filepath.Join(helperDir, "motty-update")
	if runtime.GOOS == "windows" {
		helper += ".exe"
	}
	if err := copyExecutable(exe, helper); err != nil {
		return fmt.Errorf("selfupdate: preparing the installer: %w", err)
	}
	job := Job{
		Kind: staged.Install.Kind, PID: os.Getpid(), Staged: staged.Path,
		Root: staged.Install.Root, Exe: exe, Dir: updater.Dir,
	}
	cmd := exec.Command(helper, job.Arguments()...)
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func copyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o700)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	return err
}
