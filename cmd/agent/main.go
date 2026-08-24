package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/Gilfoyle3301/gometrics/internal/agent"
	models "github.com/Gilfoyle3301/gometrics/internal/model"
	"github.com/Gilfoyle3301/gometrics/internal/shared"
	"github.com/caarlos0/env/v11"
)

type Agent struct {
	mu        sync.RWMutex
	storage   models.Storage
	pollCount int64
	server    string
}

func NewAgent(serverURL string) *Agent {
	return &Agent{
		server:  serverURL,
		storage: models.NewMemStorage(),
	}
}

func (a *Agent) updateGauge(name string, value float64) {
	_ = a.storage.Update(context.Background(), &models.Metrics{
		ID:    name,
		MType: models.Gauge,
		Value: &value,
	})
}

func (a *Agent) updateCounter(name string, delta int64) {
	_ = a.storage.Update(context.Background(), &models.Metrics{
		ID:    name,
		MType: models.Counter,
		Delta: &delta,
	})
}

func (a *Agent) collectMetrics() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.pollCount++
	m := new(runtime.MemStats)
	runtime.ReadMemStats(m)

	gauges := map[string]float64{
		"Alloc":         float64(m.Alloc),
		"BuckHashSys":   float64(m.BuckHashSys),
		"Frees":         float64(m.Frees),
		"GCCPUFraction": m.GCCPUFraction,
		"GCSys":         float64(m.GCSys),
		"HeapAlloc":     float64(m.HeapAlloc),
		"HeapIdle":      float64(m.HeapIdle),
		"HeapInuse":     float64(m.HeapInuse),
		"HeapObjects":   float64(m.HeapObjects),
		"HeapReleased":  float64(m.HeapReleased),
		"HeapSys":       float64(m.HeapSys),
		"LastGC":        float64(m.LastGC),
		"Lookups":       float64(m.Lookups),
		"MCacheInuse":   float64(m.MCacheInuse),
		"MCacheSys":     float64(m.MCacheSys),
		"MSpanInuse":    float64(m.MSpanInuse),
		"MSpanSys":      float64(m.MSpanSys),
		"Mallocs":       float64(m.Mallocs),
		"NextGC":        float64(m.NextGC),
		"NumForcedGC":   float64(m.NumForcedGC),
		"OtherSys":      float64(m.OtherSys),
		"PauseTotalNs":  float64(m.PauseTotalNs),
		"StackInuse":    float64(m.StackInuse),
		"StackSys":      float64(m.StackSys),
		"Sys":           float64(m.Sys),
		"TotalAlloc":    float64(m.TotalAlloc),
		"NumGC":         float64(m.NumGC),
		"RandomValue":   rand.Float64(),
	}

	for name, value := range gauges {
		a.updateGauge(name, value)
	}

	a.updateCounter("PollCount", 1)

	slog.Info("Collect metrics done")
}

func (a *Agent) reportMetrics(client *http.Client) {
	metrics, err := a.storage.GetAll(context.Background())
	if err != nil {
		slog.Error("failed to get all metrics", "error", err)
		return
	}

	for _, m := range metrics {
		updateURL, err := url.JoinPath(a.server, "update")
		if err != nil {
			slog.Error("failed to build url", "metric", m.ID, "error", err)
			continue
		}
		v, e := json.Marshal(m)
		if e != nil {
			slog.Error("failed to marshal metric", "metric", m.ID, "error", e)
			continue
		}

		resp, err := client.Post(updateURL, "application/json", bytes.NewBuffer(v))
		if err != nil {
			slog.Error("failed to send metric", "metric", m.ID, "error", err)
			continue
		}

		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= http.StatusBadRequest {
			slog.Error(
				"server returned bad status",
				"metric", m.ID,
				"status", resp.Status,
			)
			continue
		}

		if m.MType == models.Counter && m.Delta != nil {
			negDelta := -*m.Delta
			a.updateCounter(m.ID, negDelta)
		}
	}
}

var (
	address        *string
	reportInterval *int
	pollInterval   *int
)

func init() {
	address = flag.String("a", "http://localhost:8080", "server address")
	reportInterval = flag.Int("r", 10, "report interval in seconds")
	pollInterval = flag.Int("p", 2, "poll interval in seconds")
}

func main() {
	flag.Parse()

	cfg := agent.NewConfig()
	if err := env.Parse(cfg); err != nil {
		slog.Error("failed to parse config", "error", err)
		os.Exit(1)
	}

	addr := shared.ValueOr(cfg.Address, *address)
	pIntervalSec := shared.ValueOr(cfg.PollInterval, *pollInterval)
	rIntervalSec := shared.ValueOr(cfg.ReportInterval, *reportInterval)

	if pIntervalSec <= 0 || rIntervalSec <= 0 {
		slog.Error("intervals must be positive")
		os.Exit(1)
	}

	pInterval := time.Duration(pIntervalSec) * time.Second
	rInterval := time.Duration(rIntervalSec) * time.Second

	a := NewAgent(addr)

	collectTicker := time.NewTicker(pInterval)
	defer collectTicker.Stop()

	sendTicker := time.NewTicker(rInterval)
	defer sendTicker.Stop()

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	go func() {
		for range collectTicker.C {
			a.collectMetrics()
		}
	}()

	go func() {
		for range sendTicker.C {
			a.reportMetrics(client)
		}
	}()

	select {}
}
