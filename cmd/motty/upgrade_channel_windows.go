package main

import (
	"os"
	"path/filepath"
)

// This fork-only acceptance build uses a dedicated profile through the whole
// app/helper/restart chain, including launches from the installer's shortcut.
func init() {
	dir, err := os.UserConfigDir()
	if err != nil {
		panic(err)
	}
	if err := os.Setenv(dataDirVariable, filepath.Join(dir, "motty-upgrade-test")); err != nil {
		panic(err)
	}
}
