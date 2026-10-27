package middlware

import (
	"bytes"
	"crypto/rsa"
	"io"
	"net/http"

	"github.com/Gilfoyle3301/gometrics/internal/shared"
)

// DecryptMiddlware расшифровывает тело запроса приватным RSA-ключом.
// Мидлвар должен стоять после проверки подписи: HashSHA256 считается
// от шифртекста — тех же байт, что отправил агент. При nil-ключе
// запросы проходят без изменений.
func DecryptMiddlware(h http.Handler, key *rsa.PrivateKey) http.Handler {
	if key == nil {
		return h
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil || r.Body == http.NoBody {
			h.ServeHTTP(w, r)
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBodySize))
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		r.Body.Close()

		if len(body) == 0 {
			r.Body = io.NopCloser(bytes.NewReader(body))
			h.ServeHTTP(w, r)
			return
		}

		plain, err := shared.DecryptRSA(key, body)
		if err != nil {
			http.Error(w, "failed to decrypt body", http.StatusBadRequest)
			return
		}

		r.Body = io.NopCloser(bytes.NewReader(plain))
		r.ContentLength = int64(len(plain))

		h.ServeHTTP(w, r)
	})
}
