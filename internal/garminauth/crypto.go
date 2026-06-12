// Package garminauth is the durable, encrypted, single-row store for the
// garmin-bridge's opaque auth token blob. It holds no Garmin protocol
// knowledge: the blob is stored and returned verbatim (after decrypt). See
// the add-garmin-auth-token proposal.
package garminauth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

// ErrCryptoKey is returned when the configured encryption key is not a valid
// AES-256 (32-byte) key.
var ErrCryptoKey = errors.New("garmin token encryption key must be 32 bytes")

// crypto seals and opens token blobs with AES-256-GCM over a fixed key. A
// fresh random nonce is generated per seal and stored alongside the
// ciphertext; open requires both.
type crypto struct {
	aead cipher.AEAD
}

// newCrypto builds an AES-256-GCM sealer over the supplied 32-byte key.
func newCrypto(key []byte) (*crypto, error) {
	if len(key) != 32 {
		return nil, ErrCryptoKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	return &crypto{aead: aead}, nil
}

// seal encrypts plaintext, returning the ciphertext and the fresh nonce used.
func (c *crypto) seal(plaintext []byte) (ciphertext, nonce []byte, err error) {
	nonce = make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext = c.aead.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

// open decrypts ciphertext with the stored nonce. It fails if the ciphertext
// or nonce has been tampered with (GCM authentication).
func (c *crypto) open(ciphertext, nonce []byte) ([]byte, error) {
	if len(nonce) != c.aead.NonceSize() {
		return nil, errors.New("garmin token nonce has wrong size")
	}
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("open garmin token: %w", err)
	}
	return plaintext, nil
}
