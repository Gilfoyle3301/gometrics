package main

import (
	"encoding/json"
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

	metrics := a.storage.GetAllMetrics()
	require.NotEmpty(t, metrics)

	byName := make(map[string]any, len(metrics))
	for _, m := range metrics {
		byName[m.Name] = m.Value
	}

	for _, name := range []string{
		"Alloc", "HeapAlloc", "HeapSys", "NumGC", "RandomValue", "TotalAlloc",
	} {
		_, ok := byName[name]
		assert.Truef(t, ok, "expected gauge %q to be present", name)
	}
	pollCount, ok := byName["PollCount"]
	require.True(t, ok, "PollCount must be present")
	assert.EqualValues(t, 1, pollCount)

	rv, ok := byName["RandomValue"].(float64)
	require.True(t, ok, "RandomValue must be float64")
	assert.GreaterOrEqual(t, rv, 0.0)
	assert.Less(t, rv, 1.0)
}

func TestCollectMetricsPollCountAccumulates(t *testing.T) {
	a := NewAgent("http://localhost:8080")

	const iterations = 5
	for range iterations {
		a.collectMetrics()
	}
	pollCount, ok := a.storage.GetCounter("PollCount")
	require.True(t, ok, "PollCount must be present")
	assert.EqualValues(t, iterations, pollCount)
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
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/update", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var m models.Metrics
		require.NoError(t, json.NewDecoder(r.Body).Decode(&m))
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

		requestCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a := NewAgent(server.URL)
	a.collectMetrics()

	expectedCount := len(a.storage.GetAllMetrics())
	require.Greater(t, expectedCount, 0)

	client := &http.Client{Timeout: time.Second}
	a.reportMetrics(client)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, expectedCount, requestCount, "expected one request per collected metric")
	assert.Greater(t, gaugeCount, 0)
	assert.Greater(t, counterCount, 0)
}

func TestSendMetricsServerReturnsErrorDoesNotStopSending(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	a := NewAgent(server.URL)
	a.collectMetrics()
	expectedCount := len(a.storage.GetAllMetrics())

	client := &http.Client{Timeout: time.Second}

	assert.NotPanics(t, func() {
		a.reportMetrics(client)
	})

	assert.EqualValues(t, expectedCount, requestCount.Load(),
		"5xx response must not stop sending remaining metrics")
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
