//go:build !darwin && !windows && !linux

package main

import "log"

func main() {
	log.Fatal("motty: the interactive shell currently requires the macOS or Windows Viva backend")
}
