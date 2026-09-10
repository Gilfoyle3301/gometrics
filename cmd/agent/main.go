package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
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
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

type Agent struct {
	storage   models.Storage
	server    string
	key       string
	rateLimit int
}

func NewAgent(serverURL, key string, rateLimit int) *Agent {
	return &Agent{
		server:    serverURL,
		key:       key,
		rateLimit: rateLimit,
		storage:   models.NewMemStorage(),
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

func (a *Agent) collectRuntimeMetrics() {
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

	slog.Info("Collect runtime metrics done")
}

func (a *Agent) collectSystemMetrics() {
	vm, err := mem.VirtualMemory()
	if err != nil {
		slog.Error("failed to read virtual memory", "error", err)
	} else {
		a.updateGauge("TotalMemory", float64(vm.Total))
		a.updateGauge("FreeMemory", float64(vm.Free))
	}

	utilizations, err := cpu.Percent(0, true)
	if err != nil {
		slog.Error("failed to read cpu utilization", "error", err)
		return
	}

	for i, u := range utilizations {
		a.updateGauge(fmt.Sprintf("CPUutilization%d", i+1), u)
	}
}

func (a *Agent) sendMetric(ctx context.Context, client *http.Client, m models.Metrics) {
	body, err := json.Marshal(m)
	if err != nil {
		slog.Error("failed to marshal metric", "metric", m.ID, "error", err)
		return
	}

	updateURL, err := url.JoinPath(a.server, "update/")
	if err != nil {
		slog.Error("failed to build url", "error", err)
		return
	}

	var resp *http.Response
	err = shared.DoWithRetries(ctx, shared.RetryDelays,
		func(err error) bool { return err != nil },
		func(ctx context.Context) error {
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, updateURL, bytes.NewReader(body))
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")
			if a.key != "" {
				req.Header.Set(shared.HashHeader, shared.CalcHash(body, a.key))
			}

			r, err := client.Do(req)
			if err != nil {
				return err
			}

			_, _ = io.Copy(io.Discard, r.Body)
			r.Body.Close()

			resp = r
			return nil
		})
	if err != nil {
		slog.Error("failed to send metric after retries", "metric", m.ID, "error", err)
		return
	}

	if resp.StatusCode >= http.StatusBadRequest {
		slog.Error("server returned bad status", "metric", m.ID, "status", resp.Status)
		return
	}

	if m.MType == models.Counter && m.Delta != nil {
		a.updateCounter(m.ID, -*m.Delta)
	}
}

func (a *Agent) reportMetrics(ctx context.Context, client *http.Client) {
	metrics, err := a.storage.GetAll(ctx)
	if err != nil {
		slog.Error("failed to get all metrics", "error", err)
		return
	}

	if len(metrics) == 0 {
		return
	}

	jobs := make(chan models.Metrics, len(metrics))

	var wg sync.WaitGroup
	wg.Add(a.rateLimit)
	for range a.rateLimit {
		go func() {
			defer wg.Done()
			for m := range jobs {
				a.sendMetric(ctx, client, m)
			}
		}()
	}

	for _, m := range metrics {
		jobs <- m
	}
	close(jobs)

	wg.Wait()
}

var (
	address        *string
	reportInterval *int
	pollInterval   *int
	keyFlag        *string
	rateLimit      *int
)

func init() {
	address = flag.String("a", "http://localhost:8080", "server address")
	reportInterval = flag.Int("r", 10, "report interval in seconds")
	pollInterval = flag.Int("p", 2, "poll interval in seconds")
	keyFlag = flag.String("k", "", "secret key for data signing")
	rateLimit = flag.Int("l", 1, "max number of concurrent requests to the server")
}

func main() {
	flag.Parse()

	cfg := agent.NewConfig()
	if err := env.Parse(cfg); err != nil {
		slog.Error("failed to parse config", "error", err)
		os.Exit(1)
	}

	addr := agent.NormalizeAddress(shared.ValueOr(cfg.Address, *address))
	key := shared.ValueOr(cfg.Key, *keyFlag)
	pIntervalSec := shared.ValueOr(cfg.PollInterval, *pollInterval)
	rIntervalSec := shared.ValueOr(cfg.ReportInterval, *reportInterval)
	limit := shared.ValueOr(cfg.RateLimit, *rateLimit)

	if pIntervalSec <= 0 || rIntervalSec <= 0 {
		slog.Error("intervals must be positive")
		os.Exit(1)
	}

	if limit <= 0 {
		slog.Error("rate limit must be positive")
		os.Exit(1)
	}

	pInterval := time.Duration(pIntervalSec) * time.Second
	rInterval := time.Duration(rIntervalSec) * time.Second

	a := NewAgent(addr, key, limit)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	ctx := context.Background()

	collectTicker := time.NewTicker(pInterval)
	defer collectTicker.Stop()

	systemTicker := time.NewTicker(pInterval)
	defer systemTicker.Stop()

	sendTicker := time.NewTicker(rInterval)
	defer sendTicker.Stop()

	go func() {
		for range collectTicker.C {
			a.collectRuntimeMetrics()
		}
	}()

	go func() {
		for range systemTicker.C {
			a.collectSystemMetrics()
		}
	}()

	go func() {
		for range sendTicker.C {
			a.reportMetrics(ctx, client)
		}
	}()

	select {}
}
