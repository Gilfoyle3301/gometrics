package main

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	models "github.com/Gilfoyle3301/gometrics/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAgent(t *testing.T) {
	a := NewAgent("http://localhost:8080")
	require.NotNil(t, a.storage)
	assert.Equal(t, "http://localhost:8080", a.server)
}

func TestCollectMetrics(t *testing.T) {
	a := NewAgent("http://localhost:8080")

	a.collectMetrics()

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

func TestCollectMetricsPollCountAccumulates(t *testing.T) {
	a := NewAgent("http://localhost:8080")

	const iterations = 5
	for range iterations {
		a.collectMetrics()
	}

	m, err := a.storage.Get(context.Background(), "PollCount", models.Counter)
	require.NoError(t, err, "PollCount must be present")
	require.NotNil(t, m.Delta)
	assert.EqualValues(t, iterations, *m.Delta)
}

func TestDefaultAddressHasScheme(t *testing.T) {
	assert.Equal(t, "http://localhost:8080", *address)
}

func TestSendMetricsSuccess(t *testing.T) {
	var (
		mu           sync.Mutex
		requestCount int
		gaugeCount   int
		counterCount int
		received     []models.Metrics
		gzipEncoded  bool
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, []string{"/updates", "/updates/"}, r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body io.Reader = r.Body
		if r.Header.Get("Content-Encoding") == "gzip" {
			gzipEncoded = true
			gz, err := gzip.NewReader(r.Body)
			require.NoError(t, err)
			defer gz.Close()
			body = gz
		}

		var batch []models.Metrics
		require.NoError(t, json.NewDecoder(body).Decode(&batch))
		require.NotEmpty(t, batch)

		for _, m := range batch {
			switch m.MType {
			case models.Gauge:
				require.NotNil(t, m.Value)
				assert.Nil(t, m.Delta)
				gaugeCount++
			case models.Counter:
				require.NotNil(t, m.Delta)
				assert.Nil(t, m.Value)
				counterCount++
			default:
				t.Fatalf("unknown metric type %q", m.MType)
			}
		}

		received = batch
		requestCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a := NewAgent(server.URL)
	a.collectMetrics()

	metrics, err := a.storage.GetAll(context.Background())
	require.NoError(t, err)
	expectedCount := len(metrics)
	require.Greater(t, expectedCount, 0)

	client := &http.Client{Timeout: time.Second}
	a.reportMetrics(client)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 1, requestCount, "expected a single batch request")
	assert.Len(t, received, expectedCount, "batch must contain all collected metrics")
	assert.Greater(t, gaugeCount, 0)
	assert.Greater(t, counterCount, 0)
	assert.True(t, gzipEncoded, "batch request must be gzip-encoded")

	pollCount, err := a.storage.Get(context.Background(), "PollCount", models.Counter)
	require.NoError(t, err)
	require.NotNil(t, pollCount.Delta)
	assert.EqualValues(t, 0, *pollCount.Delta, "counters must be reset after successful send")
}

func TestSendMetricsEmptyBatchIsNotSent(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a := NewAgent(server.URL)
	client := &http.Client{Timeout: time.Second}

	a.reportMetrics(client)

	assert.EqualValues(t, 0, requestCount.Load(), "empty batch must not be sent")
}

func TestSendMetricsServerErrorKeepsCounters(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	a := NewAgent(server.URL)
	a.collectMetrics()

	client := &http.Client{Timeout: time.Second}

	assert.NotPanics(t, func() {
		a.reportMetrics(client)
	})

	assert.EqualValues(t, 1, requestCount.Load(), "expected a single batch request")

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

	a := NewAgent(server.URL)
	a.collectMetrics()

	client := &http.Client{Timeout: 1 * time.Millisecond}

	assert.NotPanics(t, func() {
		a.reportMetrics(client)
	})
}

func TestAgentConcurrentCollectAndSend(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a := NewAgent(server.URL)
	client := &http.Client{Timeout: time.Second}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(2)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				a.collectMetrics()
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
				a.reportMetrics(client)
			}
		}
	}()

	time.Sleep(200 * time.Millisecond)
	close(stop)
	wg.Wait()
}
