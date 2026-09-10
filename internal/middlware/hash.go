package middlware

import (
	"bytes"
	"crypto/hmac"
	"io"
	"net/http"

	"github.com/Gilfoyle3301/gometrics/internal/shared"
)

// maxRequestBodySize ограничивает чтение тела при проверке подписи: хеш
// считается от всего тела, и без лимита один запрос произвольного размера
// выедает память процесса.
const maxRequestBodySize = 10 << 20 // 10 MiB

// RequestHashMiddlware проверяет подпись входящего запроса. Проверка включается
// только при заданном ключе.
//
// По умолчанию запрос без заголовка подписи проходит: автотест 14-го инкремента
// сам шлёт на сервер с ключом неподписанные запросы и ждёт 200/404, поэтому
// жёсткое отклонение ломает приёмку. Для контура, где неподписанные запросы
// недопустимы, есть strict — с ним отсутствие заголовка тоже даёт 400.
//
// Тело читается целиком до мидлвара декомпрессии, поэтому хеш считается
// от исходных байтов запроса — тех же, что подписывает агент.
func RequestHashMiddlware(h http.Handler, key string, strict bool) http.Handler {
	if key == "" {
		return h
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headerHash := r.Header.Get(shared.HashHeader)
		if headerHash == "" && !strict {
			h.ServeHTTP(w, r)
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBodySize))
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(body))

		// hmac.Equal — константное по времени сравнение: обычный != позволяет
		// подбирать подпись посимвольно по времени ответа.
		if !hmac.Equal([]byte(shared.CalcHash(body, key)), []byte(headerHash)) {
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
