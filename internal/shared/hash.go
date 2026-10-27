// Package shared содержит утилиты, общие для агента и сервера:
// SHA256-подпись данных, ретраи с настраиваемыми задержками
// и выбор значения по указателю.
package shared

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashHeader — имя HTTP-заголовка с SHA256-подписью тела.
const HashHeader = "HashSHA256"

// CalcHash возвращает hex-строку SHA256 от данных с домешанным секретным ключом.
func CalcHash(data []byte, key string) string {
	h := sha256.New()
	h.Write(data)
	h.Write([]byte(key))
	return hex.EncodeToString(h.Sum(nil))
}
