package crypto

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validKey() string {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return hex.EncodeToString(key)
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	enc, err := NewAESTokenEncryptor(validKey())
	require.NoError(t, err)

	plaintext := "my-nightscout-api-secret-12345"

	ciphertext, err := enc.Encrypt(plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, ciphertext)

	decrypted, err := enc.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestEncryptProducesDifferentCiphertexts(t *testing.T) {
	enc, err := NewAESTokenEncryptor(validKey())
	require.NoError(t, err)

	c1, err := enc.Encrypt("same-secret")
	require.NoError(t, err)

	c2, err := enc.Encrypt("same-secret")
	require.NoError(t, err)

	assert.NotEqual(t, c1, c2, "AES-GCM should use random nonce, producing different ciphertexts")
}

func TestDecryptTamperedCiphertext(t *testing.T) {
	enc, err := NewAESTokenEncryptor(validKey())
	require.NoError(t, err)

	ciphertext, err := enc.Encrypt("secret")
	require.NoError(t, err)

	tampered := []byte(ciphertext)
	if len(tampered) > 5 {
		tampered[5] ^= 0xff
	}

	_, err = enc.Decrypt(string(tampered))
	assert.Error(t, err)
}

func TestNewEncryptorInvalidKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"too short", "abcd"},
		{"not hex", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"},
		{"empty", ""},
		{"31 bytes", hex.EncodeToString(make([]byte, 31))},
		{"33 bytes", hex.EncodeToString(make([]byte, 33))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewAESTokenEncryptor(tt.key)
			assert.Error(t, err)
		})
	}
}

func TestDecryptEmptyString(t *testing.T) {
	enc, err := NewAESTokenEncryptor(validKey())
	require.NoError(t, err)

	_, err = enc.Decrypt("")
	assert.Error(t, err)
}
