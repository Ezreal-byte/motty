// Package selfupdate replaces the running program with a newer release and starts
// it again. It downloads the package a release names, holds it to its checksum
// and, on the systems that sign their programs, to the signer of the running
// copy; then a helper that outlives the program swaps the files in place.
//
// Only installations the user owns are replaced: an application bundle in a
// folder the user can write, the per-user Windows installer's directory, a Linux
// prefix made by the archive's install.sh. Anything else (a .deb, Program
// Files, a portable program) is left to the caller to send to the download page.
package selfupdate

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// Kind is the way this installation is replaced.
type Kind uint8

const (
	KindNone Kind = iota
	KindMacApp
	KindWindowsSetup
	KindLinuxPrefix
)

func (kind Kind) String() string {
	switch kind {
	case KindMacApp:
		return "mac-app"
	case KindWindowsSetup:
		return "windows-setup"
	case KindLinuxPrefix:
		return "linux-prefix"
	}
	return "none"
}

func parseKind(name string) Kind {
	for _, kind := range []Kind{KindMacApp, KindWindowsSetup, KindLinuxPrefix} {
		if kind.String() == name {
			return kind
		}
	}
	return KindNone
}

// Reason says why an installation cannot replace itself. ReasonNone means it can.
type Reason uint8

const (
	ReasonNone Reason = iota
	// ReasonNotUserInstall is an installation that was not made the way the
	// packages make it: a system package, a portable program, a development build.
	ReasonNotUserInstall
	// ReasonNotWritable is a standard installation the user cannot change.
	ReasonNotWritable
	// ReasonTranslocated is a macOS app running from the temporary place the
	// system copies a downloaded, never moved app to.
	ReasonTranslocated
)

// Install is where the running program lives and how it is replaced.
type Install struct {
	Kind Kind
	// Exe is the running program, links resolved.
	Exe string
	// Root is what gets replaced: the .app bundle, the Windows install directory
	// or the Linux prefix.
	Root string
}

// Env is what Detect asks of the system, so a test can describe any machine.
type Env struct {
	GOOS         string
	LocalAppData string // %LOCALAPPDATA% on Windows
	Writable     func(dir string) bool
	Exists       func(path string) bool
}

// SystemEnv describes the machine this runs on.
func SystemEnv() Env {
	return Env{GOOS: runtime.GOOS, LocalAppData: os.Getenv("LOCALAPPDATA"), Writable: writable, Exists: exists}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// writable reports whether files can be made in dir.
func writable(dir string) bool {
	file, err := os.CreateTemp(dir, ".motty-write-*")
	if err != nil {
		return false
	}
	name := file.Name()
	file.Close()
	return os.Remove(name) == nil
}

// linuxPrefix is what install.sh accepts in a prefix.
var linuxPrefix = regexp.MustCompile(`^[A-Za-z0-9_./+-]+$`)

// Detect says how the program at exe (links resolved) is replaced, or why it
// cannot be.
func Detect(exe string, env Env) (Install, Reason) {
	install := Install{Exe: exe}
	switch env.GOOS {
	case "darwin":
		// <Name>.app/Contents/MacOS/<exe>
		macOS := filepath.Dir(exe)
		contents := filepath.Dir(macOS)
		bundle := filepath.Dir(contents)
		if filepath.Base(macOS) != "MacOS" || filepath.Base(contents) != "Contents" || !strings.HasSuffix(bundle, ".app") {
			return install, ReasonNotUserInstall
		}
		if strings.Contains(exe, "/AppTranslocation/") {
			return install, ReasonTranslocated
		}
		if !env.Writable(filepath.Dir(bundle)) {
			return install, ReasonNotWritable
		}
		install.Kind, install.Root = KindMacApp, bundle
	case "windows":
		// %LOCALAPPDATA%\Programs\<Name>\<exe>, with the installer's uninstaller beside it.
		dir := filepath.Dir(exe)
		programs := filepath.Join(env.LocalAppData, "Programs")
		if env.LocalAppData == "" || !strings.EqualFold(filepath.Dir(dir), programs) || !env.Exists(filepath.Join(dir, "Uninstall.exe")) {
			return install, ReasonNotUserInstall
		}
		if !env.Writable(dir) {
			return install, ReasonNotWritable
		}
		install.Kind, install.Root = KindWindowsSetup, dir
	case "linux":
		// <prefix>/bin/<exe>, with the menu entry install.sh makes under the prefix.
		bin := filepath.Dir(exe)
		prefix := filepath.Dir(bin)
		desktop := filepath.Join(prefix, "share", "applications", filepath.Base(exe)+".desktop")
		if filepath.Base(bin) != "bin" || !linuxPrefix.MatchString(prefix) || !env.Exists(desktop) {
			return install, ReasonNotUserInstall
		}
		if !env.Writable(bin) {
			return install, ReasonNotWritable
		}
		install.Kind, install.Root = KindLinuxPrefix, prefix
	default:
		return install, ReasonNotUserInstall
	}
	return install, ReasonNone
}
