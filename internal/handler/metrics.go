package handler

import (
	"compress/gzip"
	"encoding/json"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"

	models "github.com/Gilfoyle3301/gometrics/internal/model"
	"github.com/Gilfoyle3301/gometrics/internal/templates"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
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
	storage models.Storage
	logger  *zap.SugaredLogger
}

func New(s models.Storage) metrics {
	return metrics{storage: s}
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

func (m *metrics) UpdateMetric(w http.ResponseWriter, r *http.Request) {

	mtr := new(models.Metrics)
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "Invalid Content-Type", http.StatusBadRequest)
		return
	}
	if err := json.NewDecoder(r.Body).Decode(mtr); err != nil {
		http.Error(w, "Invalid metric data", http.StatusBadRequest)
		return
	}

	if mtr.MType != models.Counter && mtr.MType != models.Gauge {
		http.Error(w, "Unknow metrics name", http.StatusBadRequest)
		return
	}

	switch mtr.MType {
	case models.Gauge:
		if mtr.Value == nil {
			http.Error(w, "Invalid gauge value", http.StatusBadRequest)
			return
		}
		m.storage.SetGauge(mtr.ID, *mtr.Value)

	case models.Counter:
		if mtr.Delta == nil {
			http.Error(w, "Invalid counter delta", http.StatusBadRequest)
			return
		}
		m.storage.AddCounter(mtr.ID, *mtr.Delta)

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

func (m *metrics) GetMetric(w http.ResponseWriter, r *http.Request) {

	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "Content-Type header not set to application/json", http.StatusBadRequest)
		return
	}

	var body io.Reader = r.Body

	if r.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, "Failed to create gzip reader", http.StatusBadRequest)
			return
		}
		defer gz.Close()
		body = gz
	}

	mtr := new(models.GetMetrics)
	if err := json.NewDecoder(body).Decode(mtr); err != nil {
		http.Error(w, "Invalid metric data", http.StatusBadRequest)
		return
	}
	out := new(models.Metrics)
	switch mtr.MType {
	case models.Gauge:
		val, ok := m.storage.GetGauge(mtr.ID)
		if !ok {
			http.Error(w, "Gauge not found", http.StatusNotFound)
			return
		}
		out.ID = mtr.ID
		out.MType = mtr.MType
		out.Value = &val

	case models.Counter:
		val, ok := m.storage.GetCounter(mtr.ID)
		if !ok {
			http.Error(w, "Counter not found", http.StatusNotFound)
			return
		}
		out.ID = mtr.ID
		out.MType = mtr.MType
		out.Delta = &val
	default:
		http.Error(w, "Invalid metric type", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(out); err != nil {
		if m.logger != nil {
			m.logger.Error("failed to encode response: %v", err)
		}
	}

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
