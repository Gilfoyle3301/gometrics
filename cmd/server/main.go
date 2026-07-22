package main

import (
	"flag"
	"net/http"

	"github.com/Gilfoyle3301/gometrics/internal/handler"
	"github.com/Gilfoyle3301/gometrics/internal/middleware"
	models "github.com/Gilfoyle3301/gometrics/internal/model"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

func main() {
	addr := flag.String("addr", "localhost:8080", "server address")
	flag.Parse()
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()
	sg := logger.Sugar()
	r := mux.NewRouter()
	handle := handler.New(models.NewMemStorage())
	r.Handle("/update/{type}/{name}/{value}", middleware.LoggerMiddlware(http.HandlerFunc(handle.UpdateMetrics), sg)).Methods("POST")
	r.Handle("/update", middleware.LoggerMiddlware(http.HandlerFunc(handle.UpdateMetrics), sg)).Methods("POST")
	r.Handle("/value/{type}/{name}", middleware.LoggerMiddlware(http.HandlerFunc(handle.GetMetrics), sg)).Methods("GET")
	r.Handle("/value", middleware.LoggerMiddlware(http.HandlerFunc(handle.GetMetric), sg)).Methods("POST")
	r.Handle("/", middleware.LoggerMiddlware(http.HandlerFunc(handle.MainPage), sg)).Methods("GET")
	if err := http.ListenAndServe(*addr, r); err != nil {
		panic("ohhoh")
	}
}
