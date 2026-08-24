package models

import "net/http"

type ResponseRecorder struct {
	http.ResponseWriter
	Status      int
	Size        int
	wroteHeader bool
}

func (r *ResponseRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}

	r.Status = status
	r.wroteHeader = true

	r.ResponseWriter.WriteHeader(status)
}

func (r *ResponseRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(b)
	r.Size += n
	return n, err
}

func (r *ResponseRecorder) Header() http.Header {
	return r.ResponseWriter.Header()
}
