package middlware

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Gilfoyle3301/gometrics/internal/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func echoHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})
}

func TestDecryptMiddlwareNilKeyPassthrough(t *testing.T) {
	h := DecryptMiddlware(echoHandler(), nil)

	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader([]byte("plain body")))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "plain body", w.Body.String())
}

func TestDecryptMiddlwareDecryptsBody(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	encrypted, err := shared.EncryptRSA(&key.PublicKey, []byte(`{"id":"Alloc","type":"gauge"}`))
	require.NoError(t, err)

	h := DecryptMiddlware(echoHandler(), key)
	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(encrypted))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, `{"id":"Alloc","type":"gauge"}`, w.Body.String())
	assert.EqualValues(t, len(`{"id":"Alloc","type":"gauge"}`), req.ContentLength)
}

func TestDecryptMiddlwareBadCiphertext(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	h := DecryptMiddlware(echoHandler(), key)
	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader([]byte("garbage")))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDecryptMiddlwareEmptyBodyPassthrough(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	h := DecryptMiddlware(echoHandler(), key)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Body.String())
}
