package models

import (
	"context"
	"fmt"
	"sync"
)

type MemStorage struct {
	mu       sync.RWMutex
	gauges   map[string]float64
	counters map[string]int64
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

func (m *MemStorage) Update(ctx context.Context, metric *Metrics) error {
	if metric == nil {
		return fmt.Errorf("metric is nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	switch metric.MType {
	case Gauge:
		if metric.Value == nil {
			return fmt.Errorf("gauge value cannot be nil")
		}
		m.gauges[metric.ID] = *metric.Value

	case Counter:
		if metric.Delta == nil {
			return fmt.Errorf("counter delta cannot be nil")
		}
		m.counters[metric.ID] += *metric.Delta

	default:
		return fmt.Errorf("unsupported metric type: %s", metric.MType)
	}

	return nil
}

func (m *MemStorage) Get(ctx context.Context, name string, mType string) (*Metrics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	switch mType {
	case Gauge:
		if val, ok := m.gauges[name]; ok {
			return &Metrics{
				ID:    name,
				MType: Gauge,
				Value: &val,
			}, nil
		}
		return nil, fmt.Errorf("gauge metric %s not found", name)

	case Counter:
		if val, ok := m.counters[name]; ok {
			return &Metrics{
				ID:    name,
				MType: Counter,
				Delta: &val,
			}, nil
		}
		return nil, fmt.Errorf("counter metric %s not found", name)

	default:
		return nil, fmt.Errorf("unsupported metric type: %s", mType)
	}
}

func (m *MemStorage) GetAll(ctx context.Context) ([]Metrics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Metrics

	for name, value := range m.gauges {
		result = append(result, Metrics{
			ID:    name,
			MType: Gauge,
			Value: &value,
		})
	}

	for name, delta := range m.counters {
		result = append(result, Metrics{
			ID:    name,
			MType: Counter,
			Delta: &delta,
		})
	}

	return result, nil
}

func (m *MemStorage) Ping(ctx context.Context) error {
	return nil
}

func (m *MemStorage) Close() error {
	return nil
}
