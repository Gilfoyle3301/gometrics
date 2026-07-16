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
	if m.gauge == nil {
		m.gauge = make(map[string]float64)
	}
	m.gauge[name] = value
}

func (m *MemStorage) AddCounter(name string, value int64) {
	if m.counter == nil {
		m.counter = make(map[string]int64)
	}
	v, ok := m.counter[name]
	if !ok {
		m.counter[name] = value
	}
	m.counter[name] = v + value
}

type MetricForAgent struct {
	Name  string
	Type  string
	Value any
}

func (m *MemStorage) GetAllMetrics() []MetricForAgent {
	var result []MetricForAgent

	for name, value := range m.gauge {
		result = append(result, MetricForAgent{Name: name, Type: Gauge, Value: value})
	}

	for name, value := range m.counter {
		result = append(result, MetricForAgent{Name: name, Type: Counter, Value: value})
	}

	return result
}

func (m *MemStorage) GetCounter(name string) int64 {
	m.mu.RLock()
	defer m.mu.Unlock()
	return m.counter[name]
}
