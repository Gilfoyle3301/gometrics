package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	models "github.com/Gilfoyle3301/gometrics/internal/model"
	"github.com/Gilfoyle3301/gometrics/internal/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAgent(t *testing.T) {
	a := NewAgent("http://localhost:8080", "secret", 5)
	require.NotNil(t, a.storage)
	assert.Equal(t, "http://localhost:8080", a.server)
	assert.Equal(t, "secret", a.key)
	assert.Equal(t, 5, a.rateLimit)
}

func TestCollectRuntimeMetrics(t *testing.T) {
	a := NewAgent("http://localhost:8080", "", 1)

	a.collectRuntimeMetrics()

	metrics, err := a.storage.GetAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, metrics)

	byName := make(map[string]models.Metrics, len(metrics))
	for _, m := range metrics {
		byName[m.ID] = m
	}

	for _, name := range []string{
		"Alloc", "HeapAlloc", "HeapSys", "NumGC", "RandomValue", "TotalAlloc",
	} {
		m, ok := byName[name]
		assert.Truef(t, ok, "expected gauge %q to be present", name)
		if ok {
			assert.Equal(t, models.Gauge, m.MType)
			require.NotNil(t, m.Value)
		}
	}

	pollCountMetric, ok := byName["PollCount"]
	require.True(t, ok, "PollCount must be present")
	assert.Equal(t, models.Counter, pollCountMetric.MType)
	require.NotNil(t, pollCountMetric.Delta)
	assert.EqualValues(t, 1, *pollCountMetric.Delta)

	rvMetric, ok := byName["RandomValue"]
	require.True(t, ok, "RandomValue must be present")
	require.NotNil(t, rvMetric.Value)
	rv := *rvMetric.Value
	assert.GreaterOrEqual(t, rv, 0.0)
	assert.Less(t, rv, 1.0)
}

func TestCollectRuntimeMetricsPollCountAccumulates(t *testing.T) {
	a := NewAgent("http://localhost:8080", "", 1)

	const iterations = 5
	for range iterations {
		a.collectRuntimeMetrics()
	}

	m, err := a.storage.Get(context.Background(), "PollCount", models.Counter)
	require.NoError(t, err, "PollCount must be present")
	require.NotNil(t, m.Delta)
	assert.EqualValues(t, iterations, *m.Delta)
}

func TestCollectSystemMetrics(t *testing.T) {
	a := NewAgent("http://localhost:8080", "", 1)

	a.collectSystemMetrics()

	metrics, err := a.storage.GetAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, metrics)

	byName := make(map[string]models.Metrics, len(metrics))
	for _, m := range metrics {
		byName[m.ID] = m
	}

	for _, name := range []string{"TotalMemory", "FreeMemory"} {
		m, ok := byName[name]
		require.Truef(t, ok, "expected gauge %q to be present", name)
		assert.Equal(t, models.Gauge, m.MType)
		require.NotNil(t, m.Value)
		assert.GreaterOrEqual(t, *m.Value, 0.0)
	}

	totalMemory, ok := byName["TotalMemory"]
	require.True(t, ok)
	require.NotNil(t, totalMemory.Value)
	assert.Greater(t, *totalMemory.Value, 0.0)

	for i := 1; i <= runtime.NumCPU(); i++ {
		name := fmt.Sprintf("CPUutilization%d", i)
		m, ok := byName[name]
		require.Truef(t, ok, "expected gauge %q to be present", name)
		assert.Equal(t, models.Gauge, m.MType)
		require.NotNil(t, m.Value)
		assert.GreaterOrEqual(t, *m.Value, 0.0)
		assert.LessOrEqual(t, *m.Value, 100.0)
	}
}

func TestDefaultAddressHasScheme(t *testing.T) {
	assert.Equal(t, "http://localhost:8080", *address)
}

func TestDefaultRateLimit(t *testing.T) {
	assert.Equal(t, 1, *rateLimit)
}

func TestSendMetricsSuccess(t *testing.T) {
	var (
		mu           sync.Mutex
		requestCount int
		gaugeCount   int
		counterCount int
		received     []models.Metrics
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, []string{"/update", "/update/"}, r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var metric models.Metrics
		require.NoError(t, json.NewDecoder(r.Body).Decode(&metric))

		switch metric.MType {
		case models.Gauge:
			require.NotNil(t, metric.Value)
			assert.Nil(t, metric.Delta)
			gaugeCount++
		case models.Counter:
			require.NotNil(t, metric.Delta)
			assert.Nil(t, metric.Value)
			counterCount++
		default:
			t.Fatalf("unknown metric type %q", metric.MType)
		}

		received = append(received, metric)
		requestCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a := NewAgent(server.URL, "", 3)
	a.collectRuntimeMetrics()

	metrics, err := a.storage.GetAll(context.Background())
	require.NoError(t, err)
	expectedCount := len(metrics)
	require.Greater(t, expectedCount, 0)

	client := &http.Client{Timeout: time.Second}
	a.reportMetrics(context.Background(), client)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, expectedCount, requestCount, "each metric must be sent by its own request")
	assert.Greater(t, gaugeCount, 0)
	assert.Greater(t, counterCount, 0)

	sent := make(map[string]models.Metrics, len(received))
	for _, m := range received {
		sent[m.ID+":"+m.MType] = m
	}
	assert.Len(t, sent, expectedCount, "all collected metrics must be delivered exactly once")
	for _, m := range metrics {
		_, ok := sent[m.ID+":"+m.MType]
		assert.Truef(t, ok, "metric %q (%s) must be delivered", m.ID, m.MType)
	}

	pollCount, err := a.storage.Get(context.Background(), "PollCount", models.Counter)
	require.NoError(t, err)
	require.NotNil(t, pollCount.Delta)
	assert.EqualValues(t, 0, *pollCount.Delta, "counters must be reset after successful send")
}

func TestSendMetricsHashHeader(t *testing.T) {
	const key = "secret-key"

	var (
		mu    sync.Mutex
		bad   []string
		count int
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		count++
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		if r.Header.Get(shared.HashHeader) != shared.CalcHash(body, key) {
			bad = append(bad, string(body))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a := NewAgent(server.URL, key, 3)
	a.collectRuntimeMetrics()

	client := &http.Client{Timeout: time.Second}
	a.reportMetrics(context.Background(), client)

	mu.Lock()
	defer mu.Unlock()
	require.Greater(t, count, 0, "requests must carry metrics")
	assert.Empty(t, bad,
		"every request must carry the hash of the exact bytes sent, computed with the key")
}

func TestSendMetricsEmptyBatchIsNotSent(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a := NewAgent(server.URL, "", 3)
	client := &http.Client{Timeout: time.Second}

	a.reportMetrics(context.Background(), client)

	assert.EqualValues(t, 0, requestCount.Load(), "empty batch must not be sent")
}

func TestSendMetricsServerErrorKeepsCounters(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	a := NewAgent(server.URL, "", 3)
	a.collectRuntimeMetrics()

	metrics, err := a.storage.GetAll(context.Background())
	require.NoError(t, err)

	client := &http.Client{Timeout: time.Second}

	assert.NotPanics(t, func() {
		a.reportMetrics(context.Background(), client)
	})

	assert.EqualValues(t, len(metrics), requestCount.Load(),
		"each metric must be attempted once: a server error is not retried")

	pollCount, err := a.storage.Get(context.Background(), "PollCount", models.Counter)
	require.NoError(t, err)
	require.NotNil(t, pollCount.Delta)
	assert.EqualValues(t, 1, *pollCount.Delta,
		"counters must not be reset when the server returns an error")
}

func TestSendMetricsTimeoutDoesNotPanic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	origDelays := shared.RetryDelays
	shared.RetryDelays = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	defer func() { shared.RetryDelays = origDelays }()

	a := NewAgent(server.URL, "", 3)
	a.collectRuntimeMetrics()

	client := &http.Client{Timeout: 1 * time.Millisecond}

	assert.NotPanics(t, func() {
		a.reportMetrics(context.Background(), client)
	})
}

func TestSendMetricsTransportErrorRetries(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	var accepts atomic.Int32
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			accepts.Add(1)
			conn.Close()
		}
	}()

	origDelays := shared.RetryDelays
	shared.RetryDelays = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	defer func() { shared.RetryDelays = origDelays }()

	a := NewAgent("http://"+ln.Addr().String(), "", 1)
	a.updateCounter("PollCount", 1)

	client := &http.Client{Timeout: time.Second}
	a.reportMetrics(context.Background(), client)

	assert.EqualValues(t, 4, accepts.Load(), "first attempt plus three retries on transport error")
}

func TestReportMetricsLimitsConcurrency(t *testing.T) {
	const (
		limit        = 3
		metricsCount = 12
	)

	var (
		inFlight atomic.Int32
		maxSeen  atomic.Int32
		total    atomic.Int32
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := inFlight.Add(1)
		for {
			old := maxSeen.Load()
			if cur <= old || maxSeen.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		inFlight.Add(-1)
		total.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a := NewAgent(server.URL, "", limit)
	for i := range metricsCount {
		a.updateGauge(fmt.Sprintf("TestMetric%d", i), float64(i))
	}

	client := &http.Client{Timeout: time.Second}
	a.reportMetrics(context.Background(), client)

	assert.EqualValues(t, metricsCount, total.Load(), "all metrics must be delivered")
	assert.LessOrEqual(t, maxSeen.Load(), int32(limit),
		"concurrent requests must not exceed the rate limit")
	assert.Greater(t, maxSeen.Load(), int32(1),
		"requests must actually run in parallel")
}

func TestAgentConcurrentCollectAndSend(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a := NewAgent(server.URL, "", 3)
	client := &http.Client{Timeout: time.Second}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(3)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				a.collectRuntimeMetrics()
			}
		}
	}()

	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				a.collectSystemMetrics()
			}
		}
	}()

	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				a.reportMetrics(context.Background(), client)
			}
		}
	}()

	time.Sleep(200 * time.Millisecond)
	close(stop)
	wg.Wait()
}
