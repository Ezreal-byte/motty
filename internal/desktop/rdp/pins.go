package rdp

import (
	"bufio"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Pins remembers the certificate each RDP server presented when the user trusted
// it, the way a known-hosts file remembers SSH host keys: a server's certificate
// is self-signed, so what makes it trustworthy is that it is the same as the one
// the user accepted. One line per server, "address sha256:<base64>", in a file of
// its own next to the SSH known-hosts file. A server has at most one pin: trusting
// a changed certificate replaces the old one.
type Pins struct {
	path string
	mu   sync.Mutex
}

// NewPins returns the store kept in the file at path, created when the first
// certificate is remembered.
func NewPins(path string) *Pins { return &Pins{path: path} }

// Fingerprint is how a certificate is named: the SHA-256 of its DER encoding,
// in base64 without padding, as ssh-keygen prints a key's.
func Fingerprint(der []byte) string {
	sum := sha256.Sum256(der)
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
}

// Lookup returns the fingerprint pinned for address, or "" if there is none.
func (pins *Pins) Lookup(address string) (string, error) {
	pins.mu.Lock()
	defer pins.mu.Unlock()
	entries, err := pins.read()
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.address == address {
			return entry.fingerprint, nil
		}
	}
	return "", nil
}

// Remember pins fingerprint for address, replacing any earlier one.
func (pins *Pins) Remember(address, fingerprint string) error {
	if strings.ContainsAny(address, " \t\r\n") || address == "" || !strings.HasPrefix(fingerprint, "SHA256:") || strings.ContainsAny(fingerprint, " \t\r\n") {
		return fmt.Errorf("rdp: cannot pin %q for %q", fingerprint, address)
	}
	pins.mu.Lock()
	defer pins.mu.Unlock()
	entries, err := pins.read()
	if err != nil {
		return err
	}
	replaced := false
	for index := range entries {
		if entries[index].address == address {
			entries[index].fingerprint, replaced = fingerprint, true
		}
	}
	if !replaced {
		entries = append(entries, pin{address, fingerprint})
	}
	var out strings.Builder
	for _, entry := range entries {
		fmt.Fprintf(&out, "%s %s\n", entry.address, entry.fingerprint)
	}
	if err := os.MkdirAll(filepath.Dir(pins.path), 0o700); err != nil {
		return err
	}
	temporary := pins.path + ".tmp"
	if err := os.WriteFile(temporary, []byte(out.String()), 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, pins.path)
}

type pin struct{ address, fingerprint string }

// read parses the file; a line that is not an address and a fingerprint is left
// out (and dropped by the next write), and a missing file is empty.
func (pins *Pins) read() ([]pin, error) {
	file, err := os.Open(pins.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var entries []pin
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 4096), 64<<10)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && strings.HasPrefix(fields[1], "SHA256:") {
			entries = append(entries, pin{fields[0], fields[1]})
		}
	}
	return entries, scanner.Err()
}
