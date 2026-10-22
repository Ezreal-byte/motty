package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

// Certificate fixtures, enabled by environment variables:
//
//   - E2E_USER_CERT=1 signs the fixture client key as a user certificate
//     (principal "tester"), writes it beside the identity as
//     id_ed25519-cert.pub the way OpenSSH pairs them, refuses the password
//     method, and accepts only that certificate.
//   - E2E_HOST_CERT=1 presents a host certificate signed by a throwaway CA
//     and prints the CA's public key line, so the client's known_hosts can
//     carry an @cert-authority line for it.

// certFixture is the certificate state of one server run.
type certFixture struct {
	ca           ssh.Signer
	userCert     *ssh.Certificate
	userCertPath string
	extraKeys    []ssh.PublicKey
}

// prepareCerts builds the fixture state the environment asks for. clientKey is
// the fixture client key's public half, whose certificate is written beside
// the identity file.
func prepareCerts(identityPath string, clientKey ssh.PublicKey) (*certFixture, error) {
	fixture := &certFixture{}
	if path := os.Getenv("E2E_EXTRA_PUBKEY"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read E2E_EXTRA_PUBKEY: %w", err)
		}
		key, _, _, _, err := ssh.ParseAuthorizedKey(data)
		if err != nil {
			return nil, fmt.Errorf("parse E2E_EXTRA_PUBKEY: %w", err)
		}
		fixture.extraKeys = append(fixture.extraKeys, key)
	}
	if os.Getenv("E2E_USER_CERT") != "1" {
		return fixture, nil
	}
	_, caPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	ca, err := ssh.NewSignerFromKey(caPrivate)
	if err != nil {
		return nil, err
	}
	certificate := &ssh.Certificate{
		CertType:        ssh.UserCert,
		Key:             clientKey,
		ValidPrincipals: []string{"tester"},
		ValidAfter:      uint64(time.Now().Add(-time.Minute).Unix()),
		ValidBefore:     uint64(time.Now().Add(24 * time.Hour).Unix()),
	}
	if err := certificate.SignCert(rand.Reader, ca); err != nil {
		return nil, err
	}
	fixture.ca, fixture.userCert = ca, certificate
	fixture.userCertPath = identityPath + "-cert.pub"
	line := certificate.Type() + " " + base64.StdEncoding.EncodeToString(certificate.Marshal()) + " e2e-user-cert\n"
	if err := os.WriteFile(fixture.userCertPath, []byte(line), 0o600); err != nil {
		return nil, err
	}
	return fixture, nil
}

// hostKey returns the server's host key: the plain fixture key, or with
// E2E_HOST_CERT=1 a certificate for it signed by a throwaway CA, whose public
// key line is returned for the client's known_hosts.
func hostKey(plain ssh.Signer) (ssh.Signer, string, error) {
	if os.Getenv("E2E_HOST_CERT") != "1" {
		return plain, "", nil
	}
	_, caPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, "", err
	}
	ca, err := ssh.NewSignerFromKey(caPrivate)
	if err != nil {
		return nil, "", err
	}
	certificate := &ssh.Certificate{
		CertType:        ssh.HostCert,
		Key:             plain.PublicKey(),
		ValidPrincipals: []string{"127.0.0.1", "localhost"},
		ValidAfter:      uint64(time.Now().Add(-time.Minute).Unix()),
		ValidBefore:     uint64(time.Now().Add(24 * time.Hour).Unix()),
	}
	if err := certificate.SignCert(rand.Reader, ca); err != nil {
		return nil, "", err
	}
	certified, err := ssh.NewCertSigner(certificate, plain)
	if err != nil {
		return nil, "", err
	}
	line := "ssh-ed25519 " + base64.StdEncoding.EncodeToString(ca.PublicKey().Marshal())
	return certified, line, nil
}

// acceptKey says whether the presented client credential is one the fixture
// takes: an extra public key (E2E_EXTRA_PUBKEY), the fixture's plain key, or
// under E2E_USER_CERT=1 the fixture key's certificate signed by the fixture CA.
func (fixture *certFixture) acceptKey(key ssh.PublicKey) bool {
	for _, extra := range fixture.extraKeys {
		if string(extra.Marshal()) == string(key.Marshal()) {
			return true
		}
	}
	if fixture.userCert != nil {
		certificate, ok := key.(*ssh.Certificate)
		return ok &&
			certificate.CertType == ssh.UserCert &&
			string(certificate.Key.Marshal()) == string(fixture.userCert.Key.Marshal()) &&
			string(certificate.SignatureKey.Marshal()) == string(fixture.ca.PublicKey().Marshal())
	}
	return false
}
