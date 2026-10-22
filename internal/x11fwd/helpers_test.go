package x11fwd

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"

	"golang.org/x/crypto/ssh"
)

func hexDecode(s string) ([]byte, error) { return hex.DecodeString(s) }

func ed25519Key() (ed25519.PublicKey, ssh.Signer, error) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	signer, err := ssh.NewSignerFromKey(private)
	return public, signer, err
}
