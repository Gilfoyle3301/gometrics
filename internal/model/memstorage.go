package models

import "sync"

type MemStorage struct {
	mu      sync.RWMutex
	gauge   map[string]float64
	counter map[string]int64
}

type Storage interface {
	SetGauge(name string, value float64)
	AddCounter(name string, value int64)
}

func (m *MemStorage) SetGauge(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.gauge == nil {
		m.gauge = make(map[string]float64)
	}
	m.gauge[name] = value
}

func (m *MemStorage) AddCounter(name string, value int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.counter == nil {
		m.counter = make(map[string]int64)
	}
	v, ok := m.counter[name]
	if !ok {
		m.counter[name] = value
	}
	m.counter[name] = v + value
}

type MetricRow struct {
	Name  string
	Type  string
	Value any
}

func (m *MemStorage) GetAllMetrics() []MetricRow {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []MetricRow

	for name, value := range m.gauge {
		result = append(result, MetricRow{Name: name, Type: Gauge, Value: value})
	}

	for name, value := range m.counter {
		result = append(result, MetricRow{Name: name, Type: Counter, Value: value})
	}

	return result
}

func (m *MemStorage) GetCounter(name string) (int64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.counter[name]

	return v, ok
}

func (m *MemStorage) GetGauge(name string) (float64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.gauge[name]

	return v, ok
}

func MergeWithStrategy[K comparable, V any](dst, src map[K]V) map[K]V {
	// result := make(map[K]V, len(dst)+len(src))

	return dst
}
