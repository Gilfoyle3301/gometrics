package main

import (
	"net/http"

	"github.com/Gilfoyle3301/gometrics/internal/handler"
	"github.com/gorilla/mux"
)

func main() {
	r := mux.NewRouter()
	handle := handler.New()
	r.HandleFunc("/update/{type}/{name}/{value}", handle.UpdateMetrics).Methods("POST")
	r.HandleFunc("/value/{type}/{name}", handle.GetMetrics).Methods("GET")
	r.HandleFunc("/", handle.MainPage).Methods("GET")
	if err := http.ListenAndServe(":8080", r); err != nil {
		panic("ohhoh")
	}
}
