package knownhosts

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/ssh"
	sshknownhosts "golang.org/x/crypto/ssh/knownhosts"
)

func TestList(t *testing.T) {
	key := testPublicKey(t, 1)
	fingerprint := ssh.FingerprintSHA256(key)
	hashed := sshknownhosts.HashHostname("hidden.example.com")
	lines := []string{
		"# managed by a person",
		sshknownhosts.Line([]string{"plain.example.com"}, key) + " office host",
		sshknownhosts.Line([]string{"port.example.com:2200"}, key),
		sshknownhosts.Line([]string{"one.example.com", "two.example.com"}, key) + " multiple patterns stay visible",
		hashed + " " + key.Type() + " " + encodedKey(key) + " private host",
		"@revoked revoked.example.com " + key.Type() + " " + encodedKey(key) + " compromised key",
		"",
		"preserve this unknown line",
		"broken.example.com ssh-ed25519 definitely-not-base64",
	}
	path := filepath.Join(t.TempDir(), "known_hosts")
	writeFile(t, path, []byte(strings.Join(lines, "\r\n")+"\r\n"), 0o600)

	entries, err := New(path).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 5 {
		t.Fatalf("got %d entries, want 5: %#v", len(entries), entries)
	}

	tests := []struct {
		index   int
		line    int
		hosts   []string
		marker  string
		comment string
		hashed  bool
	}{
		{0, 2, []string{"plain.example.com"}, "", "office host", false},
		{1, 3, []string{"[port.example.com]:2200"}, "", "", false},
		{2, 4, []string{"one.example.com", "two.example.com"}, "", "multiple patterns stay visible", false},
		{3, 5, []string{hashed}, "", "private host", true},
		{4, 6, []string{"revoked.example.com"}, "@revoked", "compromised key", false},
	}
	for _, test := range tests {
		entry := entries[test.index]
		if entry.Line != test.line || !slices.Equal(entry.HostPatterns, test.hosts) ||
			entry.Marker != test.marker || entry.Comment != test.comment || entry.Hashed != test.hashed ||
			entry.KeyType != ssh.KeyAlgoED25519 || entry.Fingerprint != fingerprint || entry.ID == "" {
			t.Errorf("entry %d = %#v", test.index, entry)
		}
	}
}

func TestSearch(t *testing.T) {
	first := testPublicKey(t, 2)
	second := testPublicKey(t, 3)
	path := filepath.Join(t.TempDir(), "known_hosts")
	data := fmt.Sprintf("alpha.example.com %s %s Production Router\n", first.Type(), encodedKey(first)) +
		fmt.Sprintf("beta.example.com %s %s backup\n", second.Type(), encodedKey(second))
	writeFile(t, path, []byte(data), 0o600)
	file := New(path)
	entries, err := file.List()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		query string
		want  int
	}{
		{"host", "ALPHA.EXAMPLE", 1},
		{"type", "ssh-ed25519", 2},
		{"fingerprint", entries[1].Fingerprint[7:19], 1},
		{"comment", "production router", 1},
		{"missing", "database", 0},
		{"empty", " ", 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := file.Search(test.query)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != test.want {
				t.Fatalf("Search(%q) returned %d entries, want %d", test.query, len(got), test.want)
			}
		})
	}
}

func TestDeleteLinesPreservesOtherBytesAndSetsPermissions(t *testing.T) {
	key := testPublicKey(t, 4)
	remove := sshknownhosts.Line([]string{"remove.example.com"}, key) + " old\r\n"
	keep := sshknownhosts.Line([]string{"keep.example.com"}, key) + " current\r\n"
	corrupt := "broken.example.com ssh-ed25519 definitely-not-base64\r\n"
	original := []byte("# heading\r\nunknown directive\r\n" + corrupt + remove + "\r\n" + keep + "tail without newline")
	path := filepath.Join(t.TempDir(), "known_hosts")
	writeFile(t, path, original, 0o644)

	if err := New(path).DeleteLines(4); err != nil {
		t.Fatal(err)
	}
	want := []byte("# heading\r\nunknown directive\r\n" + corrupt + "\r\n" + keep + "tail without newline")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("rewritten file:\n%q\nwant:\n%q", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Windows has no permission bits; the profile directory's ACL keeps the file private.
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", info.Mode().Perm())
	}
}

func TestDeleteIDsRemainStableAcrossLineChanges(t *testing.T) {
	key := testPublicKey(t, 5)
	first := sshknownhosts.Line([]string{"first.example.com"}, key)
	second := sshknownhosts.Line([]string{"second.example.com"}, key)
	path := filepath.Join(t.TempDir(), "known_hosts")
	writeFile(t, path, []byte(first+"\n"+second+"\n"), 0o600)
	file := New(path)
	entries, err := file.List()
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, []byte("# inserted later\n"+first+"\n"+second+"\n"), 0o600)

	if err := file.DeleteIDs(entries[1].ID); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "# inserted later\n"+first+"\n" {
		t.Fatalf("got %q", got)
	}
}

func TestDeleteAllOrNothing(t *testing.T) {
	key := testPublicKey(t, 6)
	line := sshknownhosts.Line([]string{"safe.example.com"}, key) + "\n"
	path := filepath.Join(t.TempDir(), "known_hosts")
	writeFile(t, path, []byte(line), 0o600)

	if err := New(path).DeleteLines(1, 99); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != line {
		t.Fatalf("file changed to %q", got)
	}
}

func TestConcurrentDeletes(t *testing.T) {
	key := testPublicKey(t, 7)
	first := sshknownhosts.Line([]string{"one.example.com"}, key)
	second := sshknownhosts.Line([]string{"two.example.com"}, key)
	path := filepath.Join(t.TempDir(), "known_hosts")
	writeFile(t, path, []byte(first+"\n"+second+"\n"), 0o600)
	file := New(path)
	entries, err := file.List()
	if err != nil {
		t.Fatal(err)
	}

	var wait sync.WaitGroup
	errorsOut := make(chan error, 2)
	for _, entry := range entries {
		wait.Add(1)
		go func(id ID) {
			defer wait.Done()
			errorsOut <- file.DeleteIDs(id)
		}(entry.ID)
	}
	wait.Wait()
	close(errorsOut)
	for err := range errorsOut {
		if err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("file = %q, want empty", got)
	}
}

func TestAtomicReplaceFailurePreservesOriginal(t *testing.T) {
	key := testPublicKey(t, 8)
	line := sshknownhosts.Line([]string{"safe.example.com"}, key) + "\n"
	path := filepath.Join(t.TempDir(), "known_hosts")
	writeFile(t, path, []byte(line), 0o600)
	file := New(path)

	failure := errors.New("injected rename failure")
	err := file.delete(func(Entry) bool { return true }, 1, func(_, _ string) error { return failure })
	if !errors.Is(err, failure) {
		t.Fatalf("error = %v, want injected failure", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != line {
		t.Fatalf("file changed to %q", got)
	}
	matches, globErr := filepath.Glob(filepath.Join(filepath.Dir(path), ".known_hosts-*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain: %v", matches)
	}
}

func TestReplaceRejectsChangedSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "known_hosts")
	current := []byte("# changed by another writer\n")
	writeFile(t, path, current, 0o600)

	err := replace(path, []byte("# stale snapshot\n"), []byte("replacement\n"), os.Rename)
	if !errors.Is(err, ErrChanged) {
		t.Fatalf("error = %v, want ErrChanged", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(got, current) {
		t.Fatalf("file changed to %q", got)
	}
}

func testPublicKey(t *testing.T, seed byte) ssh.PublicKey {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))
	key, err := ssh.NewPublicKey(private.Public())
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func encodedKey(key ssh.PublicKey) string {
	return base64.StdEncoding.EncodeToString(key.Marshal())
}

func writeFile(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}
