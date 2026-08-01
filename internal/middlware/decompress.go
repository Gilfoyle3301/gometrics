package middlware

import (
	"compress/gzip"
	"net/http"
)

func Decompress(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Encoding") == "gzip" {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "Failed to create gzip reader", http.StatusBadRequest)
				return
			}
			defer gz.Close()
			r.Body = gz
		}

		h.ServeHTTP(w, r)
	})
}
