package middlware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Gilfoyle3301/gometrics/internal/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestHashMiddlware(t *testing.T) {
	const key = "secret"

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		_, _ = w.Write(body)
	})

	t.Run("valid hash passes and body stays intact", func(t *testing.T) {
		h := RequestHashMiddlware(inner, key, false)

		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("payload"))
		req.Header.Set(shared.HashHeader, shared.CalcHash([]byte("payload"), key))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "payload", rec.Body.String())
	})

	t.Run("invalid hash is rejected", func(t *testing.T) {
		h := RequestHashMiddlware(inner, key, false)

		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("payload"))
		req.Header.Set(shared.HashHeader, "deadbeef")
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	// Поведение зафиксировано автотестом 14-го инкремента: он шлёт на сервер
	// с ключом запросы без HashSHA256 и ждёт 200/404. Жёсткое отклонение
	// неподписанных запросов по умолчанию ломает приёмку — для него есть strict.
	t.Run("request without hash header passes unless strict mode is on", func(t *testing.T) {
		h := RequestHashMiddlware(inner, key, false)

		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("payload"))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "payload", rec.Body.String())
	})

	t.Run("strict mode rejects request without hash header", func(t *testing.T) {
		h := RequestHashMiddlware(inner, key, true)

		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("payload"))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("strict mode accepts valid hash", func(t *testing.T) {
		h := RequestHashMiddlware(inner, key, true)

		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("payload"))
		req.Header.Set(shared.HashHeader, shared.CalcHash([]byte("payload"), key))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "payload", rec.Body.String())
	})

	t.Run("empty key disables the check", func(t *testing.T) {
		h := RequestHashMiddlware(inner, "", false)

		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("payload"))
		req.Header.Set(shared.HashHeader, "deadbeef")
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("oversized body is rejected", func(t *testing.T) {
		h := RequestHashMiddlware(inner, key, false)

		body := strings.Repeat("x", maxRequestBodySize+1)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req.Header.Set(shared.HashHeader, shared.CalcHash([]byte(body), key))
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestResponseHashMiddlware(t *testing.T) {
	const key = "secret"

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	t.Run("signs response and preserves status and headers", func(t *testing.T) {
		h := ResponseHashMiddlware(inner, key)

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
		assert.Equal(t, shared.CalcHash([]byte(`{"ok":true}`), key), rec.Header().Get(shared.HashHeader))
		assert.Equal(t, `{"ok":true}`, rec.Body.String())
	})

	t.Run("empty key adds no header", func(t *testing.T) {
		h := ResponseHashMiddlware(inner, "")

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

		assert.Empty(t, rec.Header().Get(shared.HashHeader))
	})
}
