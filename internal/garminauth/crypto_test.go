package garminauth

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testKey() []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i)
	}
	return k
}

func TestCrypto_RoundTrip(t *testing.T) {
	c, err := newCrypto(testKey())
	require.NoError(t, err)

	plaintext := []byte("garth-serialized-oauth-token-blob")
	ct, nonce, err := c.seal(plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, ct, "ciphertext must not equal plaintext")

	got, err := c.open(ct, nonce)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(plaintext, got), "decrypted blob must match input")
}

func TestCrypto_FreshNoncePerSeal(t *testing.T) {
	c, err := newCrypto(testKey())
	require.NoError(t, err)
	_, n1, err := c.seal([]byte("x"))
	require.NoError(t, err)
	_, n2, err := c.seal([]byte("x"))
	require.NoError(t, err)
	assert.False(t, bytes.Equal(n1, n2), "each seal must use a fresh nonce")
}

func TestCrypto_TamperDetected(t *testing.T) {
	c, err := newCrypto(testKey())
	require.NoError(t, err)
	ct, nonce, err := c.seal([]byte("sensitive"))
	require.NoError(t, err)

	ct[0] ^= 0xff // flip a bit
	_, err = c.open(ct, nonce)
	assert.Error(t, err, "tampered ciphertext must fail authentication")
}

func TestCrypto_WrongKeyFails(t *testing.T) {
	c1, err := newCrypto(testKey())
	require.NoError(t, err)
	ct, nonce, err := c1.seal([]byte("secret"))
	require.NoError(t, err)

	other := testKey()
	other[0] = 0xff
	c2, err := newCrypto(other)
	require.NoError(t, err)
	_, err = c2.open(ct, nonce)
	assert.Error(t, err, "decryption under a different key must fail")
}

func TestNewCrypto_RejectsWrongKeySize(t *testing.T) {
	_, err := newCrypto(make([]byte, 16))
	assert.ErrorIs(t, err, ErrCryptoKey)
}
