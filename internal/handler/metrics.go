package handler

import (
	"html/template"
	"log/slog"
	"net/http"
	"sort"
	"strconv"

	models "github.com/Gilfoyle3301/gometrics/internal/model"
	"github.com/Gilfoyle3301/gometrics/internal/templates"
	"github.com/gorilla/mux"
)

var pageTemplate = template.Must(
	template.ParseFS(templates.TemplFS, "dashboard.html"),
)

const expectedParts = 3

type PageData struct {
	Metrics      []models.MetricRow
	Total        int
	GaugeCount   int
	CounterCount int
}

type metrics struct {
	storage *models.MemStorage
}

func New() metrics {
	return metrics{storage: new(models.MemStorage)}
}

func (m *metrics) UpdateMetrics(w http.ResponseWriter, r *http.Request) {

	if r.Header.Get("Content-Type") != "text/plain" {
		http.Error(w, "Invalid Content-Type", http.StatusBadRequest)
		return
	}
	// parts := strings.Split(r.URL.Path, "/")
	parts := mux.Vars(r)
	if len(parts) != expectedParts {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	metricType := parts["type"]
	metricValueStr := parts["value"]

	metricName, ok := parts["name"]

	if !ok {
		http.Error(w, "Metric name is empty", http.StatusNotFound)
		return
	}

	if metricType != models.Counter && metricType != models.Gauge {
		http.Error(w, "Unknow metrics name", http.StatusBadRequest)
		return
	}

	var err error
	switch metricType {
	case models.Gauge:
		var value float64
		value, err = strconv.ParseFloat(metricValueStr, 64)
		if err != nil {
			http.Error(w, "Invalid gauge value", http.StatusBadRequest)
			return
		}
		m.storage.SetGauge(metricName, value)

	case models.Counter:
		var value int64

		value, err = strconv.ParseInt(metricValueStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid counter value", http.StatusBadRequest)
			return
		}
		m.storage.AddCounter(metricName, value)

	default:
		http.Error(w, "Invalid metric type", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))

}

func (m *metrics) GetMetrics(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	metricType := vars["type"]
	metricName := vars["name"]

	var valueStr string
	// found := false

	switch metricType {
	case models.Gauge:
		val, ok := m.storage.GetGauge(metricName)
		if !ok {
			http.Error(w, "Gauge not found", http.StatusNotFound)
			return
		}
		valueStr = strconv.FormatFloat(val, 'f', -1, 64)
	case models.Counter:
		val, ok := m.storage.GetCounter(metricName)
		if !ok {
			http.Error(w, "Counter not found", http.StatusNotFound)
			return
		}
		valueStr = strconv.FormatInt(val, 10)
	default:
		http.Error(w, "Invalid metric type", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(valueStr))

}

func (m *metrics) MainPage(w http.ResponseWriter, r *http.Request) {
	slog.Info("Get main page")
	allMetrics := m.storage.GetAllMetrics()
	data := PageData{
		Metrics: make([]models.MetricRow, 0, len(allMetrics)),
	}
	for _, mt := range allMetrics {
		data.Metrics = append(data.Metrics, models.MetricRow{
			Name:  mt.Name,
			Type:  mt.Type,
			Value: mt.Value,
		})
		switch mt.Type {
		case models.Gauge:
			data.GaugeCount++
		case models.Counter:
			data.CounterCount++
		}
	}

	sort.Slice(data.Metrics, func(i, j int) bool {
		return data.Metrics[i].Name < data.Metrics[j].Name
	})

	data.Total = len(data.Metrics)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pageTemplate.Execute(w, data); err != nil {
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
		return
	}

}
