package handler

import (
	"encoding/json"
	"html/template"
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

type Handler struct {
	storage models.Storage
	logger  *zap.SugaredLogger
}

func New(s models.Storage, logger *zap.SugaredLogger) *Handler {
	return &Handler{
		storage: s,
		logger:  logger,
	}
}

func (h *Handler) UpdateMetrics(w http.ResponseWriter, r *http.Request) {
	parts := mux.Vars(r)
	if len(parts) != expectedParts {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	metricType := parts["type"]
	metricName := parts["name"]
	metricValueStr := parts["value"]

	metric := &models.Metrics{
		ID:    metricName,
		MType: metricType,
	}

	switch metricType {
	case models.Gauge:
		value, err := strconv.ParseFloat(metricValueStr, 64)
		if err != nil {
			http.Error(w, "Invalid gauge value", http.StatusBadRequest)
			return
		}
		metric.Value = &value

	case models.Counter:
		delta, err := strconv.ParseInt(metricValueStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid counter value", http.StatusBadRequest)
			return
		}
		metric.Delta = &delta

	default:
		http.Error(w, "Unknown metric type", http.StatusBadRequest)
		return
	}

	if err := h.storage.Update(r.Context(), metric); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (h *Handler) UpdateMetric(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "Invalid Content-Type", http.StatusBadRequest)
		return
	}

	var metric models.Metrics
	if err := json.NewDecoder(r.Body).Decode(&metric); err != nil {
		http.Error(w, "Invalid metric data", http.StatusBadRequest)
		return
	}

	if metric.MType != models.Gauge && metric.MType != models.Counter {
		http.Error(w, "Unknown metric type", http.StatusBadRequest)
		return
	}

	if metric.MType == models.Gauge && metric.Value == nil {
		http.Error(w, "Invalid gauge value", http.StatusBadRequest)
		return
	}
	if metric.MType == models.Counter && metric.Delta == nil {
		http.Error(w, "Invalid counter delta", http.StatusBadRequest)
		return
	}

	if err := h.storage.Update(r.Context(), &metric); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (h *Handler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	metricType := vars["type"]
	metricName := vars["name"]

	if metricType != models.Gauge && metricType != models.Counter {
		http.Error(w, "Invalid metric type", http.StatusBadRequest)
		return
	}

	metric, err := h.storage.Get(r.Context(), metricName, metricType)
	if err != nil {
		http.Error(w, "Metric not found", http.StatusNotFound)
		return
	}

	var valueStr string
	switch metric.MType {
	case models.Gauge:
		if metric.Value != nil {
			valueStr = strconv.FormatFloat(*metric.Value, 'f', -1, 64)
		}
	case models.Counter:
		if metric.Delta != nil {
			valueStr = strconv.FormatInt(*metric.Delta, 10)
		}
	default:
		http.Error(w, "Invalid metric type", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(valueStr))
}

func (h *Handler) GetMetric(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
		return
	}

	var req models.Metrics
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid metric data", http.StatusBadRequest)
		return
	}

	metric, err := h.storage.Get(r.Context(), req.ID, req.MType)
	if err != nil {
		http.Error(w, "Metric not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(metric); err != nil {
		if h.logger != nil {
			h.logger.Error("failed to encode response: %v", zap.Error(err))
		}
	}
}

func (h *Handler) MainPage(w http.ResponseWriter, r *http.Request) {
	slog.Info("Get main page")

	allMetrics, err := h.storage.GetAll(r.Context())
	if err != nil {
		http.Error(w, "Failed to load metrics", http.StatusInternalServerError)
		return
	}

	data := PageData{
		Metrics: make([]models.MetricRow, 0, len(allMetrics)),
	}

	for _, mt := range allMetrics {

		row := models.MetricRow{
			Name: mt.ID,
			Type: mt.MType,
		}

		switch mt.MType {
		case models.Gauge:
			if mt.Value != nil {
				row.Value = mt.Value
			}
			data.GaugeCount++
		case models.Counter:
			if mt.Delta != nil {
				row.Delta = mt.Delta
			}
			data.CounterCount++
		}

		data.Metrics = append(data.Metrics, row)
	}

	sort.Slice(data.Metrics, func(i, j int) bool {
		return data.Metrics[i].Name < data.Metrics[j].Name
	})

	data.Total = len(data.Metrics)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pageTemplate.Execute(w, data); err != nil {
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
	}
}
