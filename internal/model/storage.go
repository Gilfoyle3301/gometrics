package models

import (
	"context"
)

type Storage interface {
	Update(ctx context.Context, m *Metrics) error
	Get(ctx context.Context, name string, mType string) (*Metrics, error)
	GetAll(ctx context.Context) ([]Metrics, error)
	Ping(ctx context.Context) error
	Close() error
}
