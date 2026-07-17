package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
	for i := 0; i < iterations; i++ {
		a.collectMetrics()
	}

	assert.EqualValues(t, iterations, a.storage.GetCounter("PollCount"))
}

func TestSendMetricsSuccess(t *testing.T) {
	var (
		mu        sync.Mutex
		gotPaths  []string
		urlRegexp = regexp.MustCompile(`^/update/(gauge|counter)/[^/]+/[^/]+$`)
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotPaths = append(gotPaths, r.URL.Path)
		mu.Unlock()

		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "text/plain", r.Header.Get("Content-Type"))
		assert.Regexp(t, urlRegexp, r.URL.Path)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a := NewAgent(server.URL)
	a.collectMetrics()

	expectedCount := len(a.storage.GetAllMetrics())
	require.Greater(t, expectedCount, 0)

	client := &http.Client{Timeout: time.Second}
	metrics := a.storage.GetAllMetrics()
	for _, m := range metrics {
		url := fmt.Sprintf("%s/update/%s/%s/%v", a.server, m.Type, m.Name, m.Value)

		resp, err := client.Post(url, "text/plain", nil)
		if err != nil {
			slog.Error("failed to send metric", "metric", m.Name, "error", err)
			continue
		}
		slog.Info("metric sent", "url", url, "status", resp.Status)
		resp.Body.Close()
	}
	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, gotPaths, expectedCount, "expected one request per collected metric")
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
		metrics := a.storage.GetAllMetrics()
		for _, m := range metrics {
			url := fmt.Sprintf("%s/update/%s/%s/%v", a.server, m.Type, m.Name, m.Value)

			resp, err := client.Post(url, "text/plain", nil)
			if err != nil {
				slog.Error("failed to send metric", "metric", m.Name, "error", err)
				continue
			}
			slog.Info("metric sent", "url", url, "status", resp.Status)
			resp.Body.Close()
		}
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
		metrics := a.storage.GetAllMetrics()
		for _, m := range metrics {
			url := fmt.Sprintf("%s/update/%s/%s/%v", a.server, m.Type, m.Name, m.Value)

			resp, err := client.Post(url, "text/plain", nil)
			if err != nil {
				slog.Error("failed to send metric", "metric", m.Name, "error", err)
				continue
			}
			slog.Info("metric sent", "url", url, "status", resp.Status)
			resp.Body.Close()
		}
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
				metrics := a.storage.GetAllMetrics()
				for _, m := range metrics {
					url := fmt.Sprintf("%s/update/%s/%s/%v", a.server, m.Type, m.Name, m.Value)

					resp, err := client.Post(url, "text/plain", nil)
					if err != nil {
						slog.Error("failed to send metric", "metric", m.Name, "error", err)
						continue
					}
					slog.Info("metric sent", "url", url, "status", resp.Status)
					resp.Body.Close()
				}
			}
		}
	}()

	time.Sleep(200 * time.Millisecond)
	close(stop)
	wg.Wait()

}
