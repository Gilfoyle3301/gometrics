package models

import (
	"context"
)

// Storage — интерфейс хранилища метрик. Единый контракт для всех
// реализаций (память, файл, база данных).
type Storage interface {
	// Update сохраняет одну метрику: gauge перезаписывается,
	// counter накапливает дельту.
	Update(ctx context.Context, m *Metrics) error
	// UpdateBatch сохраняет набор метрик одной операцией.
	UpdateBatch(ctx context.Context, batch []Metrics) error
	// Get возвращает метрику по имени и типу или ошибку, если её нет.
	Get(ctx context.Context, name string, mType string) (*Metrics, error)
	// GetAll возвращает все сохранённые метрики.
	GetAll(ctx context.Context) ([]Metrics, error)
	// Ping проверяет доступность хранилища.
	Ping(ctx context.Context) error
	// Close освобождает ресурсы и выполняет финальный сброс данных.
	Close() error
}
