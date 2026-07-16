package models

type MemStorage struct {
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
