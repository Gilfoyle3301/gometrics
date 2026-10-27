package handler_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/gorilla/mux"

	"github.com/Gilfoyle3301/gometrics/internal/handler"
	models "github.com/Gilfoyle3301/gometrics/internal/model"
)

func newExampleServer() *httptest.Server {
	h := handler.New(models.NewMemStorage(), nil)

	r := mux.NewRouter()
	r.Handle("/update/{type}/{name}/{value}", http.HandlerFunc(h.UpdateMetrics)).Methods(http.MethodPost)
	r.Handle("/update", http.HandlerFunc(h.UpdateMetric)).Methods(http.MethodPost)
	r.Handle("/updates", http.HandlerFunc(h.UpdateMetricsBatch)).Methods(http.MethodPost)
	r.Handle("/value/{type}/{name}", http.HandlerFunc(h.GetMetrics)).Methods(http.MethodGet)
	r.Handle("/value", http.HandlerFunc(h.GetMetric)).Methods(http.MethodPost)

	return httptest.NewServer(r)
}

func printResponse(resp *http.Response) {
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Println(resp.StatusCode, strings.TrimSpace(string(body)))
}

func ExampleHandler_UpdateMetrics() {
	srv := newExampleServer()
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/update/gauge/Alloc/123.45", "text/plain", nil)
	if err != nil {
		panic(err)
	}
	printResponse(resp)
	// Output: 200 OK
}

func ExampleHandler_UpdateMetric() {
	srv := newExampleServer()
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/update", "application/json",
		strings.NewReader(`{"id":"Alloc","type":"gauge","value":123.45}`))
	if err != nil {
		panic(err)
	}
	printResponse(resp)
	// Output: 200 OK
}

func ExampleHandler_UpdateMetricsBatch() {
	srv := newExampleServer()
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/updates", "application/json",
		strings.NewReader(`[{"id":"Alloc","type":"gauge","value":1},{"id":"PollCount","type":"counter","delta":2}]`))
	if err != nil {
		panic(err)
	}
	printResponse(resp)
	// Output: 200 OK
}

func ExampleHandler_GetMetrics() {
	srv := newExampleServer()
	defer srv.Close()

	seed, err := http.Post(srv.URL+"/update/gauge/Alloc/123.45", "text/plain", nil)
	if err != nil {
		panic(err)
	}
	seed.Body.Close()

	resp, err := http.Get(srv.URL + "/value/gauge/Alloc")
	if err != nil {
		panic(err)
	}
	printResponse(resp)
	// Output: 200 123.45
}

func ExampleHandler_GetMetric() {
	srv := newExampleServer()
	defer srv.Close()

	seed, err := http.Post(srv.URL+"/update", "application/json",
		strings.NewReader(`{"id":"Alloc","type":"gauge","value":123.45}`))
	if err != nil {
		panic(err)
	}
	seed.Body.Close()

	resp, err := http.Post(srv.URL+"/value", "application/json",
		strings.NewReader(`{"id":"Alloc","type":"gauge"}`))
	if err != nil {
		panic(err)
	}
	printResponse(resp)
	// Output: 200 {"id":"Alloc","type":"gauge","value":123.45}
}
