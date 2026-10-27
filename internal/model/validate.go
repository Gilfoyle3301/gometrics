package models

import "fmt"

// ValidateMetric проверяет, что тип метрики поддерживается и заполнено
// соответствующее ему значение: Value для gauge, Delta для counter.
func ValidateMetric(m *Metrics) error {
	if m == nil {
		return fmt.Errorf("metric is nil")
	}

	switch m.MType {
	case Gauge:
		if m.Value == nil {
			return fmt.Errorf("gauge value cannot be nil")
		}
	case Counter:
		if m.Delta == nil {
			return fmt.Errorf("counter delta cannot be nil")
		}
	default:
		return fmt.Errorf("unsupported metric type: %s", m.MType)
	}

	return nil
}
