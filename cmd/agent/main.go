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

// const pollInterval = 2
// const reportInterval = 10

type Agent struct {
	mu        sync.RWMutex
	storage   *models.MemStorage
	pollCount int64
	server    string
}

func NewAgent(url string) *Agent {
	return &Agent{
		server:  url,
		storage: models.NewMemStorage(),
	}
}

type metric struct {
	Name  string
	Type  string
	Value any
}

func (a *Agent) collectMetrics() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pollCount++
	m := new(runtime.MemStats)
	runtime.ReadMemStats(m)

	a.storage.Update("Alloc", float64(m.Alloc))
	a.storage.Update("BuckHashSys", float64(m.BuckHashSys))
	a.storage.Update("Frees", float64(m.Frees))
	a.storage.Update("GCCPUFraction", float64(m.GCCPUFraction))
	a.storage.Update("GCSys", float64(m.GCSys))
	a.storage.Update("HeapAlloc", float64(m.HeapAlloc))
	a.storage.Update("HeapIdle", float64(m.HeapIdle))
	a.storage.Update("HeapInuse", float64(m.HeapInuse))
	a.storage.Update("HeapObjects", float64(m.HeapObjects))
	a.storage.Update("HeapReleased", float64(m.HeapReleased))
	a.storage.Update("HeapSys", float64(m.HeapSys))
	a.storage.Update("LastGC", float64(m.LastGC))
	a.storage.Update("Lookups", float64(m.Lookups))
	a.storage.Update("MCacheInuse", float64(m.MCacheInuse))
	a.storage.Update("MCacheSys", float64(m.MCacheSys))
	a.storage.Update("MSpanInuse", float64(m.MSpanInuse))
	a.storage.Update("MSpanSys", float64(m.MSpanSys))
	a.storage.Update("Mallocs", float64(m.Mallocs))
	a.storage.Update("NextGC", float64(m.NextGC))
	a.storage.Update("NumForcedGC", float64(m.NumForcedGC))
	a.storage.Update("OtherSys", float64(m.OtherSys))
	a.storage.Update("PauseTotalNs", float64(m.PauseTotalNs))
	a.storage.Update("StackInuse", float64(m.StackInuse))
	a.storage.Update("StackSys", float64(m.StackSys))
	a.storage.Update("Sys", float64(m.Sys))
	a.storage.Update("TotalAlloc", float64(m.TotalAlloc))
	a.storage.Update("NumGC", float64(m.NumGC))
	a.storage.Update("RandomValue", rand.Float64())
	a.storage.Update("PollCount", 1)

	slog.Info("Collect metrics done")

}

func (a *Agent) reportMetrics(client *http.Client) {
	metrics, err := a.storage.GetAll(context.Background())
	if err != nil {
		slog.Error("failed to get all metrics", "error", err)
		return
	}

	for _, m := range metrics {
		updateURL, err := url.JoinPath(
			a.server,
			"update",
		)
		if err != nil {
			slog.Error("failed to build url", "metric", m.Name, "error", err)
			continue
		}
		metric := models.Metrics{ID: m.Name, MType: m.Type}
		switch m.Type {
		case models.Gauge:
			value, ok := m.Value.(float64)
			if !ok {
				slog.Error("invalid gauge value", "metric", m.Name)
				continue
			}
			metric.Value = &value
		case models.Counter:
			delta, ok := m.Value.(int64)
			if !ok {
				slog.Error("invalid counter value", "metric", m.Name)
				continue
			}
			metric.Delta = &delta
		default:
			slog.Error("invalid metric type", "metric", m.Name, "type", m.Type)
			continue
		}

		v, e := json.Marshal(metric)
		if e != nil {
			slog.Error("failed to marshal metric", "metric", m.Name, "error", e)
			continue
		}
		resp, err := client.Post(updateURL, "application/json", bytes.NewBuffer(v))
		if err != nil {
			slog.Error("failed to send metric", "metric", m.Name, "error", err)
			continue
		}

		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= http.StatusBadRequest {
			slog.Error(
				"server returned bad status",
				"metric", m.Name,
				"status", resp.Status,
			)
			continue
		}

		if m.Type == models.Counter {
			a.storage.SetCounter(m.Name, 0)
		}
	}
}

var (
	address        *string
	reportInterval *time.Duration
	pollInterval   *time.Duration
)

func init() {
	address = flag.String("a", "http://localhost:8080", "server address")
	reportInterval = flag.Duration("r", 10*time.Second, "report interval")
	pollInterval = flag.Duration("p", 2*time.Second, "poll interval")

}

func main() {
	flag.Parse()

	cfg := agent.NewConfig()
	if err := env.Parse(cfg); err != nil {
		slog.Error("failed to parse config", "error", err)
		os.Exit(1)
	}

	address := shared.ValueOr(cfg.Address, *address)
	pollInterval := shared.ValueOr(cfg.PollInterval, *pollInterval)
	reportInterval := shared.ValueOr(cfg.ReportInterval, *reportInterval)

	if pollInterval <= 0 || reportInterval <= 0 {
		slog.Error("intervals must be positive")
		os.Exit(1)
	}

	a := NewAgent(address)

	collectTicker := time.NewTicker(pollInterval)
	defer collectTicker.Stop()

	sendTicker := time.NewTicker(reportInterval)
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
