package shared

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalcHash(t *testing.T) {
	t.Run("deterministic", func(t *testing.T) {
		assert.Equal(t, CalcHash([]byte("body"), "key"), CalcHash([]byte("body"), "key"))
	})

	t.Run("different keys produce different hashes", func(t *testing.T) {
		assert.NotEqual(t, CalcHash([]byte("body"), "key1"), CalcHash([]byte("body"), "key2"))
	})

	t.Run("different bodies produce different hashes", func(t *testing.T) {
		assert.NotEqual(t, CalcHash([]byte("body1"), "key"), CalcHash([]byte("body2"), "key"))
	})

	t.Run("is hex sha256 of data concatenated with key", func(t *testing.T) {
		sum := sha256.Sum256([]byte("body" + "key"))
		assert.Equal(t, hex.EncodeToString(sum[:]), CalcHash([]byte("body"), "key"))
	})
}
