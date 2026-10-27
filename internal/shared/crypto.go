package shared

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// LoadPublicKey читает публичный RSA-ключ в PEM-формате (PKIX) из файла.
func LoadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read public key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("public key file %s does not contain PEM data", path)
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not RSA")
	}
	return rsaPub, nil
}

// LoadPrivateKey читает приватный RSA-ключ в PEM-формате (PKCS#1 или PKCS#8)
// из файла.
func LoadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read private key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("private key file %s does not contain PEM data", path)
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	rsaKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not RSA")
	}
	return rsaKey, nil
}

// EncryptRSA шифрует данные публичным ключом блоками по схеме RSA-OAEP
// (SHA-256): RSA не принимает сообщения длиннее размера ключа минус
// 2*hashLen+2 байта, поэтому вход разрезается на блоки, каждый шифруется
// отдельно, результат конкатенируется. Пустые данные дают пустой шифртекст.
//
// PKCS#1 v1.5 сознательно не используется: в Go 1.26 схема объявлена
// небезопасной и deprecated, агент и сервер договариваются об OAEP.
func EncryptRSA(pub *rsa.PublicKey, data []byte) ([]byte, error) {
	blockSize := pub.Size() - 2*sha256.Size - 2
	if blockSize <= 0 {
		return nil, fmt.Errorf("rsa key is too small for encryption")
	}

	encrypted := make([]byte, 0, (len(data)/blockSize+1)*pub.Size())
	for start := 0; start < len(data); start += blockSize {
		end := min(start+blockSize, len(data))

		block, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pub, data[start:end], nil)
		if err != nil {
			return nil, fmt.Errorf("encrypt block: %w", err)
		}
		encrypted = append(encrypted, block...)
	}
	return encrypted, nil
}

// DecryptRSA расшифровывает данные приватным ключом: вход должен быть
// нарезан на блоки размером с ключ, каждый блок расшифровывается
// по схеме RSA-OAEP (SHA-256) — той же, что использует EncryptRSA.
func DecryptRSA(priv *rsa.PrivateKey, data []byte) ([]byte, error) {
	if len(data) == 0 {
		return []byte{}, nil
	}

	blockSize := priv.Size()
	if len(data)%blockSize != 0 {
		return nil, fmt.Errorf("ciphertext length %d is not a multiple of key size %d", len(data), blockSize)
	}

	decrypted := make([]byte, 0, len(data))
	for start := 0; start < len(data); start += blockSize {
		block := data[start : start+blockSize]

		plain, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, priv, block, nil)
		if err != nil {
			return nil, fmt.Errorf("decrypt block: %w", err)
		}
		decrypted = append(decrypted, plain...)
	}
	return decrypted, nil
}
