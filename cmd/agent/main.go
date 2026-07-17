package main

import (
	"flag"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"runtime"
	"sync"
	"time"

	models "github.com/Gilfoyle3301/gometrics/internal/model"
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
		storage: new(models.MemStorage),
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

	a.storage.SetGauge("Alloc", float64(m.Alloc))
	a.storage.SetGauge("BuckHashSys", float64(m.BuckHashSys))
	a.storage.SetGauge("Frees", float64(m.Frees))
	a.storage.SetGauge("GCCPUFraction", float64(m.GCCPUFraction))
	a.storage.SetGauge("GCSys", float64(m.GCSys))
	a.storage.SetGauge("HeapAlloc", float64(m.HeapAlloc))
	a.storage.SetGauge("HeapIdle", float64(m.HeapIdle))
	a.storage.SetGauge("HeapInuse", float64(m.HeapInuse))
	a.storage.SetGauge("HeapObjects", float64(m.HeapObjects))
	a.storage.SetGauge("HeapReleased", float64(m.HeapReleased))
	a.storage.SetGauge("HeapSys", float64(m.HeapSys))
	a.storage.SetGauge("LastGC", float64(m.LastGC))
	a.storage.SetGauge("Lookups", float64(m.Lookups))
	a.storage.SetGauge("MCacheInuse", float64(m.MCacheInuse))
	a.storage.SetGauge("MCacheSys", float64(m.MCacheSys))
	a.storage.SetGauge("MSpanInuse", float64(m.MSpanInuse))
	a.storage.SetGauge("MSpanSys", float64(m.MSpanSys))
	a.storage.SetGauge("Mallocs", float64(m.Mallocs))
	a.storage.SetGauge("NextGC", float64(m.NextGC))
	a.storage.SetGauge("NumForcedGC", float64(m.NumForcedGC))
	a.storage.SetGauge("OtherSys", float64(m.OtherSys))
	a.storage.SetGauge("PauseTotalNs", float64(m.PauseTotalNs))
	a.storage.SetGauge("StackInuse", float64(m.StackInuse))
	a.storage.SetGauge("StackSys", float64(m.StackSys))
	a.storage.SetGauge("Sys", float64(m.Sys))
	a.storage.SetGauge("TotalAlloc", float64(m.TotalAlloc))
	a.storage.SetGauge("RandomValue", rand.Float64())
	a.storage.AddCounter("NumGC", int64(m.NumGC)-a.getPrevNumGC())
	a.storage.AddCounter("PollCount", 1)

	slog.Info("Collect metrics done")

}

func (a *Agent) getPrevNumGC() int64 {
	v, ok := a.storage.GetCounter("NumGC")
	if !ok {
		return 0
	}
	return v
}

var (
	address        *string
	reportInterval *time.Duration
	pollInterval   *time.Duration
)

func init() {
	address = flag.String("a", "localhost:8080", "server address")
	reportInterval = flag.Duration("r", 10*time.Second, "report interval")
	pollInterval = flag.Duration("p", 2*time.Second, "poll interval")

}

func main() {
	flag.Parse()
	agent := NewAgent(*address)
	collectTicker := time.NewTicker(*pollInterval)
	sendTicker := time.NewTicker(*reportInterval)
	client := http.Client{}

	go func() {
		for range collectTicker.C {
			agent.collectMetrics()
			slog.Info("update metrics done")
		}
	}()

	go func() {
		for range sendTicker.C {
			metrics := agent.storage.GetAllMetrics()
			for _, m := range metrics {
				url := fmt.Sprintf("%s/update/%s/%s/%v", agent.server, m.Type, m.Name, m.Value)
				resp, err := client.Post(url, "text/plain", nil)
				if err != nil {
					slog.Error("Failed send metrics", "metric", m.Name, "error", err)
					continue
				}

				resp.Body.Close()
			}
		}
	}()

	select {}

}
