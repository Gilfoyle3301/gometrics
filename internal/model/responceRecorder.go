package models

import "net/http"

// ResponseRecorder — обёртка над http.ResponseWriter, которая запоминает
// статус и размер ответа для логирования запросов.
type ResponseRecorder struct {
	http.ResponseWriter
	Status      int
	Size        int
	wroteHeader bool
}

// WriteHeader фиксирует статус ответа; повторные вызовы игнорируются.
func (r *ResponseRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}

	r.Status = status
	r.wroteHeader = true

	r.ResponseWriter.WriteHeader(status)
}

// Write пишет данные в исходный ResponseWriter, подсчитывая размер ответа.
// Если WriteHeader ещё не вызывался, фиксирует статус 200.
func (r *ResponseRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(b)
	r.Size += n
	return n, err
}

// Header возвращает заголовки исходного ResponseWriter.
func (r *ResponseRecorder) Header() http.Header {
	return r.ResponseWriter.Header()
}
