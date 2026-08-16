package models

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func float64Ptr(v float64) *float64 {
	p := new(float64)
	*p = v
	return p
}

func TestMemStorageGauge(t *testing.T) {
	s := &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}

	err := s.Update(
		context.Background(),
		&Metrics{
			ID:    "Alloc",
			MType: Gauge,
			Value: float64Ptr(1.5),
		},
	)
	require.NoError(t, err)

	val, ok := s.GetGauge("Alloc")
	require.True(t, ok)
	assert.Equal(t, 1.5, val)
}

func TestMemStorageCounter(t *testing.T) {
	s := &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}

	s.AddCounter("PollCount", 2)
	s.AddCounter("PollCount", 3)

	val, ok := s.GetCounter("PollCount")
	require.True(t, ok)
	assert.Equal(t, int64(5), val)
}

func TestMemStorageGetAllMetrics(t *testing.T) {
	s := &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}

	s.SetGauge("Alloc", 1.5)
	s.AddCounter("PollCount", 2)

	metrics := s.GetAllMetrics()
	require.Len(t, metrics, 2)
}

func TestMemStorageRestore(t *testing.T) {
	s := &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}

	data := []byte(`[
		{"id":"Alloc","type":"gauge","value":1.5},
		{"id":"PollCount","type":"counter","delta":3}
	]`)

	require.NoError(t, s.Restore(data))

	gauge, ok := s.GetGauge("Alloc")
	require.True(t, ok)
	assert.Equal(t, 1.5, gauge)

	counter, ok := s.GetCounter("PollCount")
	require.True(t, ok)
	assert.Equal(t, int64(3), counter)
}

func TestResponseRecorder(t *testing.T) {
	w := httptest.NewRecorder()
	rec := &ResponseRecorder{
		ResponseWriter: w,
		Status:         http.StatusOK,
	}

	rec.WriteHeader(http.StatusCreated)

	n, err := rec.Write([]byte("OK"))
	require.NoError(t, err)

	assert.Equal(t, 2, n)
	assert.Equal(t, http.StatusCreated, rec.Status)
	assert.Equal(t, 2, rec.Size)
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}
