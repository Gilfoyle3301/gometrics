package shared

import (
	"crypto/sha256"
	"encoding/hex"
)

const HashHeader = "HashSHA256"

func CalcHash(data []byte, key string) string {
	h := sha256.New()
	h.Write(data)
	h.Write([]byte(key))
	return hex.EncodeToString(h.Sum(nil))
}
