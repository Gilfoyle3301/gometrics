package models

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// MemStorage — потокобезопасное хранилище метрик в памяти процесса.
type MemStorage struct {
	mu       sync.RWMutex
	gauges   map[string]float64
	counters map[string]int64
}

// NewMemStorage создаёт пустое хранилище метрик в памяти.
func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

// Update валидирует и сохраняет одну метрику: gauge перезаписывается,
// counter увеличивает накопленное значение на дельту.
func (m *MemStorage) Update(ctx context.Context, metric *Metrics) error {
	if err := ValidateMetric(metric); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.applyLocked(metric)

	return nil
}

func (m *MemStorage) applyLocked(metric *Metrics) {
	switch metric.MType {
	case Gauge:
		m.gauges[metric.ID] = *metric.Value
	case Counter:
		m.counters[metric.ID] += *metric.Delta
	}
}

// UpdateBatch валидирует и сохраняет все метрики пакета под одной блокировкой.
func (m *MemStorage) UpdateBatch(ctx context.Context, batch []Metrics) error {
	for i := range batch {
		if err := ValidateMetric(&batch[i]); err != nil {
			return err
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range batch {
		m.applyLocked(&batch[i])
	}

	return nil
}

// Get возвращает метрику по имени и типу или ошибку, если метрика не найдена.
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

// GetAll возвращает снимок всех сохранённых метрик обоих типов.
func (m *MemStorage) GetAll(ctx context.Context) ([]Metrics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Metrics, 0, len(m.gauges)+len(m.counters))

	for name, value := range m.gauges {
		result = append(result, Metrics{
			ID:    name,
			MType: Gauge,
			Value: &value,
		})
	}

	for name, delta := range m.counters {
		delta := delta
		result = append(result, Metrics{
			ID:    name,
			MType: Counter,
			Delta: &delta,
		})
	}

	return result, nil
}

// Ping для хранилища в памяти всегда успешен.
func (m *MemStorage) Ping(ctx context.Context) error {
	return nil
}

// Close не выполняет действий: данные живут только в памяти.
func (m *MemStorage) Close() error {
	return nil
}

// SetGauge устанавливает значение gauge-метрики.
func (m *MemStorage) SetGauge(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gauges[name] = value
}

// AddCounter увеличивает накопленное значение counter-метрики на дельту.
func (m *MemStorage) AddCounter(name string, delta int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[name] += delta
}

// GetGauge возвращает текущее значение gauge-метрики и признак её наличия.
func (m *MemStorage) GetGauge(name string) (float64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.gauges[name]
	return value, ok
}

// GetCounter возвращает накопленное значение counter-метрики и признак её наличия.
func (m *MemStorage) GetCounter(name string) (int64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.counters[name]
	return value, ok
}

// GetAllMetrics возвращает все метрики без контекста и ошибки.
func (m *MemStorage) GetAllMetrics() []Metrics {
	metrics, _ := m.GetAll(context.Background())
	return metrics
}

// Restore загружает метрики из JSON-снимка, заменяя текущее состояние:
// counter'ы присваиваются, а не инкрементируются, чтобы повторная загрузка
// файла не удваивала их значения.
func (m *MemStorage) Restore(data []byte) error {
	var metrics []Metrics

	if err := json.Unmarshal(data, &metrics); err != nil {
		return err
	}

	for _, metric := range metrics {
		if metric.ID == "" {
			return fmt.Errorf("metric id cannot be empty")
		}
		switch metric.MType {
		case Gauge:
			if metric.Value == nil {
				return fmt.Errorf("gauge value cannot be nil")
			}
		case Counter:
			if metric.Delta == nil {
				return fmt.Errorf("counter delta cannot be nil")
			}
		default:
			return fmt.Errorf("unsupported metric type: %s", metric.MType)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, metric := range metrics {
		switch metric.MType {
		case Gauge:
			m.gauges[metric.ID] = *metric.Value
		case Counter:
			m.counters[metric.ID] = *metric.Delta
		}
	}

	return nil
}
