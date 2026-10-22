package sshconn

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
)

// maxCertificateSize bounds a certificate file read from disk.
const maxCertificateSize = 1 << 20

// certificateSigner wraps signer in the OpenSSH user certificate that
// accompanies its key file, the way OpenSSH pairs id_ed25519 with
// id_ed25519-cert.pub: same path, "-cert.pub" appended. Without a companion
// file the signer is returned unchanged. A companion that is not a user
// certificate for exactly this key is an error, so a mismatch is reported
// instead of silently authenticating with the bare key.
func certificateSigner(path string, signer ssh.Signer) (ssh.Signer, error) {
	data, err := os.ReadFile(path + "-cert.pub")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return signer, nil
		}
		return nil, fmt.Errorf("read certificate %s-cert.pub: %w", path, err)
	}
	if len(data) > maxCertificateSize {
		return nil, fmt.Errorf("certificate %s-cert.pub exceeds 1 MiB", path)
	}
	key, _, _, _, err := ssh.ParseAuthorizedKey(data)
	if err != nil {
		return nil, fmt.Errorf("parse certificate %s-cert.pub: %w", path, err)
	}
	certificate, ok := key.(*ssh.Certificate)
	if !ok {
		return nil, fmt.Errorf("%s-cert.pub is a plain public key, not a certificate", path)
	}
	if certificate.CertType != ssh.UserCert {
		return nil, fmt.Errorf("%s-cert.pub is not a user certificate", path)
	}
	certified, err := ssh.NewCertSigner(certificate, signer)
	if err != nil {
		return nil, fmt.Errorf("use certificate %s-cert.pub: %w", path, err)
	}
	return certified, nil
}
