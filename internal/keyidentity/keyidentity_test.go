package keyidentity

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func writeIdentity(t testing.TB, passphrase string) (string, ssh.Signer) {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var block *pem.Block
	if passphrase == "" {
		block, err = ssh.MarshalPrivateKey(private, "")
	} else {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(private, "", []byte(passphrase))
	}
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	return path, signer
}

func TestInspectAndLoadPrivateKeys(t *testing.T) {
	for _, test := range []struct {
		name       string
		passphrase string
		encrypted  bool
	}{
		{name: "plain"},
		{name: "encrypted", passphrase: "correct horse", encrypted: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			path, want := writeIdentity(t, test.passphrase)
			info, err := Inspect(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Path != path || info.Name != "id_ed25519" || info.Algorithm != ssh.KeyAlgoED25519 ||
				info.Fingerprint != ssh.FingerprintSHA256(want.PublicKey()) || info.Encrypted != test.encrypted {
				t.Fatalf("info = %+v", info)
			}
			var passphrase []byte
			if test.passphrase != "" {
				passphrase = []byte(test.passphrase)
			}
			got, err := Load(path, passphrase)
			if err != nil {
				t.Fatalf("Load = %v, %v", got, err)
			}
			if err := got.PublicKey().Verify([]byte("probe"), sign(t, want, []byte("probe"))); err != nil {
				t.Fatalf("loaded key did not verify: %v", err)
			}
		})
	}
}

func TestInspectRejectsInvalidAndOversizedFiles(t *testing.T) {
	directory := t.TempDir()
	invalid := filepath.Join(directory, "invalid")
	if err := os.WriteFile(invalid, []byte("not a private key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(invalid); err == nil {
		t.Fatal("invalid file was accepted")
	}
	large := filepath.Join(directory, "large")
	if err := os.WriteFile(large, make([]byte, maxPrivateKeySize+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(large); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("large error = %v", err)
	}
	if _, err := Inspect(directory); err == nil {
		t.Fatal("directory was accepted")
	}
}

func TestWrongPassphraseFails(t *testing.T) {
	path, _ := writeIdentity(t, "right")
	if _, err := Load(path, []byte("wrong")); err == nil {
		t.Fatal("wrong passphrase was accepted")
	}
}

func BenchmarkInspect(b *testing.B) {
	path, _ := writeIdentity(b, "benchmark")
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Inspect(path); err != nil {
			b.Fatal(err)
		}
	}
}

func sign(t testing.TB, signer ssh.Signer, message []byte) *ssh.Signature {
	t.Helper()
	signature, err := signer.Sign(rand.Reader, message)
	if err != nil {
		t.Fatal(err)
	}
	return signature
}

// testSSHString wraps one SSH wire string for the PPK test fixtures.
func testSSHString(b []byte) []byte {
	out := make([]byte, 4+len(b))
	binary.BigEndian.PutUint32(out, uint32(len(b)))
	copy(out[4:], b)
	return out
}

// writePPK writes a minimal unencrypted PPK version 2 file for one key.
func writePPK(t testing.TB, private ed25519.PrivateKey, public ed25519.PublicKey) string {
	t.Helper()
	pubBlob := append(testSSHString([]byte("ssh-ed25519")), testSSHString(public)...)
	blobSection := func(name string, data []byte) string {
		lines := chunk(base64.StdEncoding.EncodeToString(data))
		return name + ": " + strconv.Itoa(strings.Count(lines, "\n")) + "\n" + lines
	}
	body := "PuTTY-User-Key-File-2: ssh-ed25519\nEncryption: none\nComment: ppk\n" +
		blobSection("Public-Lines", pubBlob) +
		blobSection("Private-Lines", private[:32]) +
		"Private-MAC: " + ppkMAC(pubBlob, private[:32]) + "\n"
	path := filepath.Join(t.TempDir(), "key.ppk")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func chunk(encoded string) string {
	var out strings.Builder
	for len(encoded) > 64 {
		out.WriteString(encoded[:64])
		out.WriteString("\n")
		encoded = encoded[64:]
	}
	out.WriteString(encoded)
	out.WriteString("\n")
	return out.String()
}

func ppkMAC(public, private []byte) string {
	input := testSSHString([]byte("ssh-ed25519"))
	input = append(input, testSSHString([]byte("none"))...)
	input = append(input, testSSHString([]byte("ppk"))...)
	input = append(input, testSSHString(public)...)
	input = append(input, testSSHString(private)...)
	mac := sha1.Sum([]byte("putty-private-key-file-mac-key"))
	digest := hmac.New(sha1.New, mac[:])
	digest.Write(input)
	return hex.EncodeToString(digest.Sum(nil))
}

// TestLoadReadsPPK proves a PuTTY key file loads through the same entry point
// as an OpenSSH identity, and Inspect describes it.
func TestLoadReadsPPK(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	path := writePPK(t, private, public)
	signer, err := Load(path, nil)
	if err != nil {
		t.Fatalf("load ppk: %v", err)
	}
	if signer.PublicKey().Type() != "ssh-ed25519" {
		t.Fatalf("ppk loaded as %q", signer.PublicKey().Type())
	}
	info, err := Inspect(path)
	if err != nil {
		t.Fatalf("inspect ppk: %v", err)
	}
	if info.Encrypted || info.Algorithm != "ssh-ed25519" {
		t.Fatalf("ppk info: %+v", info)
	}
}

// TestLoadNamesSecurityKeys proves an sk- identity fails with the limit named.
func TestLoadNamesSecurityKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "id_sk")
	if err := os.WriteFile(path, []byte("-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAA\ntest body mentioning sk-ssh-ed25519@openssh.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path, nil)
	if err == nil || !strings.Contains(err.Error(), "hardware security keys") {
		t.Fatalf("want a named security-key error, got %v", err)
	}
}
