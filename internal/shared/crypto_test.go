package shared

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

func writePEM(t *testing.T, dir, name, pemType string, der []byte) string {
	t.Helper()

	path := filepath.Join(dir, name)
	data := pem.EncodeToMemory(&pem.Block{Type: pemType, Bytes: der})
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path
}

func TestLoadPublicKeyPKIX(t *testing.T) {
	key := generateTestKey(t)

	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)

	pub, err := LoadPublicKey(writePEM(t, t.TempDir(), "pub.pem", "PUBLIC KEY", der))
	require.NoError(t, err)
	assert.True(t, pub.Equal(&key.PublicKey))
}

func TestLoadPrivateKeyPKCS1(t *testing.T) {
	key := generateTestKey(t)

	path := writePEM(t, t.TempDir(), "priv.pem", "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(key))
	loaded, err := LoadPrivateKey(path)
	require.NoError(t, err)
	assert.True(t, loaded.Equal(key))
}

func TestLoadPrivateKeyPKCS8(t *testing.T) {
	key := generateTestKey(t)

	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)

	loaded, err := LoadPrivateKey(writePEM(t, t.TempDir(), "priv8.pem", "PRIVATE KEY", der))
	require.NoError(t, err)
	assert.True(t, loaded.Equal(key))
}

func TestLoadKeyErrors(t *testing.T) {
	dir := t.TempDir()

	_, err := LoadPublicKey(filepath.Join(dir, "missing.pem"))
	assert.Error(t, err)

	_, err = LoadPrivateKey(filepath.Join(dir, "missing.pem"))
	assert.Error(t, err)

	badPath := filepath.Join(dir, "bad.pem")
	require.NoError(t, os.WriteFile(badPath, []byte("not a pem"), 0o600))

	_, err = LoadPublicKey(badPath)
	assert.Error(t, err)

	_, err = LoadPrivateKey(badPath)
	assert.Error(t, err)
}

func TestEncryptDecryptRSARoundtrip(t *testing.T) {
	key := generateTestKey(t)

	tests := []struct {
		name string
		data []byte
	}{
		{"small single block", []byte(`{"id":"Alloc","type":"gauge","value":123.45}`)},
		{"multi-chunk payload", make([]byte, 1000)},
		{"empty payload", []byte{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := EncryptRSA(&key.PublicKey, tt.data)
			require.NoError(t, err)

			decrypted, err := DecryptRSA(key, encrypted)
			require.NoError(t, err)
			assert.Equal(t, tt.data, decrypted)
		})
	}
}

func TestDecryptRSAAcceptsRawOAEPBlock(t *testing.T) {
	key := generateTestKey(t)

	data := []byte("oaep payload")
	encrypted, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, &key.PublicKey, data, nil)
	require.NoError(t, err)

	decrypted, err := DecryptRSA(key, encrypted)
	require.NoError(t, err)
	assert.Equal(t, data, decrypted)
}

func TestDecryptRSAErrors(t *testing.T) {
	key := generateTestKey(t)

	_, err := DecryptRSA(key, []byte("garbage"))
	assert.Error(t, err, "length not a multiple of key size must fail")

	_, err = DecryptRSA(key, make([]byte, key.Size()))
	assert.Error(t, err, "undecryptable block must fail")
}
