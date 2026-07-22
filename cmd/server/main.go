package main

import (
	"flag"
	"net/http"

	"github.com/Gilfoyle3301/gometrics/internal/handler"
	models "github.com/Gilfoyle3301/gometrics/internal/model"
	"github.com/gorilla/mux"
)

func main() {
	addr := flag.String("addr", "localhost:8080", "server address")
	flag.Parse()

	r := mux.NewRouter()
	handle := handler.New(models.NewMemStorage())
	r.HandleFunc("/update/{type}/{name}/{value}", handle.UpdateMetrics).Methods("POST")
	r.HandleFunc("/value/{type}/{name}", handle.GetMetrics).Methods("GET")
	r.HandleFunc("/", handle.MainPage).Methods("GET")
	if err := http.ListenAndServe(*addr, r); err != nil {
		panic("ohhoh")
	}
}
