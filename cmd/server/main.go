package main

import (
	"net/http"

	"github.com/Gilfoyle3301/gometrics/internal/handler"
)

func main() {
	router := http.NewServeMux()
	handle := handler.New()
	router.HandleFunc("POST /update/{type}/{name}/{value}", handle.UpdateMetrics)

	if err := http.ListenAndServe(":8080", router); err != nil {
		panic("ohhoh")
	}
}
