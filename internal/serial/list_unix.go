//go:build linux || darwin

package serial

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Ports lists the serial ports this machine has: USB and built-in UARTs on
// Linux, the call-out devices on macOS.
func Ports() []Port {
	var patterns []string
	if runtime.GOOS == "darwin" {
		patterns = []string{"/dev/cu.*"}
	} else {
		patterns = []string{"/dev/ttyUSB*", "/dev/ttyACM*", "/dev/ttyAMA*", "/dev/ttyS*", "/dev/ttyTHS*"}
	}
	var ports []Port
	seen := map[string]bool{}
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		sort.Strings(matches)
		for _, device := range matches {
			if seen[device] || skipDevice(device) {
				continue
			}
			seen[device] = true
			ports = append(ports, Port{Device: device, Description: describe(device)})
		}
	}
	return ports
}

// skipDevice drops devices that are not a port in practice: macOS's Bluetooth
// incoming port, and the Linux ttyS entries no hardware backs (there are dozens).
func skipDevice(device string) bool {
	if runtime.GOOS == "darwin" {
		return strings.Contains(device, "Bluetooth-Incoming-Port")
	}
	name := filepath.Base(device)
	if strings.HasPrefix(name, "ttyS") {
		_, err := os.Stat("/sys/class/tty/" + name + "/device/driver")
		return err != nil
	}
	return false
}

// describe gives a USB adapter its name from /dev/serial/by-id when Linux made one.
func describe(device string) string {
	links, _ := filepath.Glob("/dev/serial/by-id/*")
	for _, link := range links {
		if target, err := filepath.EvalSymlinks(link); err == nil && target == device {
			return strings.TrimPrefix(filepath.Base(link), "usb-")
		}
	}
	return ""
}
