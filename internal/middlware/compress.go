package middlware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

type gunWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

func (g gunWriter) Write(p []byte) (int, error) {
	return g.Writer.Write(p)
}

func GunZipMiddlware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			h.ServeHTTP(w, r)
			return
		}

		gz, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
		if err != nil {
			io.WriteString(w, err.Error())
			return
		}
		defer gz.Close()

		w.Header().Set("Content-Encoding", "gzip")
		h.ServeHTTP(gunWriter{ResponseWriter: w, Writer: gz}, r)
	})
}
