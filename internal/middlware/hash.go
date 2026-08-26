package middlware

import (
	"bytes"
	"io"
	"net/http"

	"github.com/Gilfoyle3301/gometrics/internal/shared"
)

// RequestHashMiddlware проверяет подпись входящего запроса.
// Проверка срабатывает, только если задан ключ и в запросе есть заголовок
// подписи: неподписанные запросы продолжают обрабатываться ради обратной
// совместимости со старыми клиентами.
// Тело читается целиком до мидлвара декомпрессии, поэтому хеш считается
// от исходных байтов запроса — тех же, что подписывает агент.
func RequestHashMiddlware(h http.Handler, key string) http.Handler {
	if key == "" {
		return h
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headerHash := r.Header.Get(shared.HashHeader)
		if headerHash == "" {
			h.ServeHTTP(w, r)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(body))

		if shared.CalcHash(body, key) != headerHash {
			http.Error(w, "bad hash", http.StatusBadRequest)
			return
		}

		h.ServeHTTP(w, r)
	})
}

// ResponseHashMiddlware подписывает тело ответа заголовком подписи.
// Ответ буферизуется целиком: хеш можно посчитать только от всего тела,
// а заголовки приходится отправлять раньше тела.
func ResponseHashMiddlware(h http.Handler, key string) http.Handler {
	if key == "" {
		return h
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bw := newHashBufferWriter()
		h.ServeHTTP(bw, r)

		dst := w.Header()
		for k, vv := range bw.header {
			dst[k] = vv
		}
		dst.Set(shared.HashHeader, shared.CalcHash(bw.body.Bytes(), key))

		w.WriteHeader(bw.status)
		_, _ = w.Write(bw.body.Bytes())
	})
}

// hashBufferWriter накапливает ответ, позволяя отложить отправку
// до вычисления хеша всего тела.
type hashBufferWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func newHashBufferWriter() *hashBufferWriter {
	return &hashBufferWriter{
		header: make(http.Header),
		status: http.StatusOK,
	}
}

func (b *hashBufferWriter) Header() http.Header {
	return b.header
}

func (b *hashBufferWriter) Write(p []byte) (int, error) {
	return b.body.Write(p)
}

func (b *hashBufferWriter) WriteHeader(code int) {
	b.status = code
}
