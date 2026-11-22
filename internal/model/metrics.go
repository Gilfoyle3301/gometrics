// Package models определяет модель метрик, её валидацию и хранилища:
// в памяти, файловое с периодическим сбросом и на основе PostgreSQL.
package models

// Поддерживаемые типы метрик.
const (
	Counter = "counter"
	Gauge   = "gauge"
)

// Metrics — модель одной метрики, переносимая между агентом и сервером.
//
// NOTE: Не усложняем пример, вводя иерархическую вложенность структур.
// Органичиваясь плоской моделью.
// Delta и Value объявлены через указатели,
// что бы отличать значение "0", от не заданного значения
// и соответственно не кодировать в структуру.
type Metrics struct {
	ID    string   `json:"id"`
	MType string   `json:"type"`
	Delta *int64   `json:"delta,omitempty"`
	Value *float64 `json:"value,omitempty"`
	Hash  string   `json:"hash,omitempty"`
}

// GetMetrics — тело запроса POST /value: метрика запрашивается по имени и типу.
type GetMetrics struct {
	ID    string `json:"id"`
	MType string `json:"type"`
}

// MetricRow — строка таблицы метрик на главной HTML-странице сервера.
type MetricRow struct {
	Name  string   `json:"name"`
	Type  string   `json:"type"`
	Delta *int64   `json:"delta,omitempty"`
	Value *float64 `json:"value"`
}
