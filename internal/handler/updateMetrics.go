package handler

import (
	"net/http"
	"strconv"
	"strings"

	models "github.com/Gilfoyle3301/gometrics/internal/model"
)

const expectedParts = 5

// type partsPath struct {
// 	metricType     string
// 	metricName     string
// 	metricValueStr string
// }

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
	parts := strings.Split(r.URL.Path, "/")

	if len(parts) != expectedParts {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	metricType := parts[2]
	metricName := parts[3]
	metricValueStr := parts[4]

	if metricName == "" {
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
