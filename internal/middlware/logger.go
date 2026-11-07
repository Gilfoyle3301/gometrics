// Package middlware содержит HTTP-мидлвары сервера: логирование запросов,
// gzip-сжатие ответов и распаковку тел, проверку и простановку SHA256-подписи.
package middlware

import (
	"net/http"
	"time"

	models "github.com/Gilfoyle3301/gometrics/internal/model"
	"go.uber.org/zap"
)

// LoggerMiddlware логирует каждый запрос: метод, путь, статус, размер ответа,
// длительность и данные клиента.
func LoggerMiddlware(h http.Handler, logger *zap.SugaredLogger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rec := &models.ResponseRecorder{ResponseWriter: w, Status: http.StatusOK}

		h.ServeHTTP(rec, r)

		logger.Infow(
			"http request",
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
			"status", rec.Status,
			"bytes", rec.Size,
			"duration", time.Since(start),
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		)
	})
}
