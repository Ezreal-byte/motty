//go:build !windows

package localshell

// InstalledWindowsShells is only available on Windows.
func InstalledWindowsShells() []Choice { return nil }
