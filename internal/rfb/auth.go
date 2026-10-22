package rfb

import "crypto/des"

// vncResponse answers a VNC Authentication challenge: the 16 bytes encrypted,
// as two DES blocks, under a key made of the password's first eight bytes with
// the bits of every byte reversed (a quirk of the original implementation that
// every server copied). DES with a key of at most 56 secret bits, a password cut
// to eight characters, and a challenge that the server chooses and that crosses
// the network in the clear make this scheme weak: anyone who can watch one
// exchange can search for the password offline. Tunnel the connection through
// SSH (see docs/remote-desktop.md) rather than rely on it.
func vncResponse(challenge [16]byte, password string) [16]byte {
	var key [8]byte
	copy(key[:], password)
	for index, value := range key {
		var reversed byte
		for bit := 0; bit < 8; bit++ {
			reversed |= (value >> bit & 1) << (7 - bit)
		}
		key[index] = reversed
	}
	block, err := des.NewCipher(key[:])
	if err != nil {
		return [16]byte{} // not reachable: the key has the length DES wants
	}
	var response [16]byte
	block.Encrypt(response[0:8], challenge[0:8])
	block.Encrypt(response[8:16], challenge[8:16])
	return response
}
