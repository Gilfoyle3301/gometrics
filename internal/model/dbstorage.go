package models

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ Storage = (*DBStorage)(nil)

type DBStorage struct {
	pool *pgxpool.Pool
}

func NewDBStorage(pool *pgxpool.Pool) *DBStorage {
	return &DBStorage{pool: pool}
}

const upsertMetricSQL = `
INSERT INTO metrics (key, type, value, delta)
VALUES ($1, $2, $3, $4)
ON CONFLICT (key) DO UPDATE SET
    type = EXCLUDED.type,
    value = CASE WHEN EXCLUDED.type = 'gauge' THEN EXCLUDED.value ELSE metrics.value END,
    delta = CASE WHEN EXCLUDED.type = 'counter' THEN COALESCE(metrics.delta, 0) + EXCLUDED.delta ELSE metrics.delta END
`

const getMetricSQL = `SELECT value, delta FROM metrics WHERE key = $1 AND type = $2`

const getAllMetricsSQL = `SELECT key, type, value, delta FROM metrics ORDER BY key`

func (s *DBStorage) Update(ctx context.Context, m *Metrics) error {
	if err := ValidateMetric(m); err != nil {
		return err
	}

	if _, err := s.pool.Exec(ctx, upsertMetricSQL, m.ID, m.MType, m.Value, m.Delta); err != nil {
		return fmt.Errorf("upsert metric %s: %w", m.ID, err)
	}

	return nil
}

func (s *DBStorage) UpdateBatch(ctx context.Context, batch []Metrics) error {
	if len(batch) == 0 {
		return nil
	}

	for i := range batch {
		if err := ValidateMetric(&batch[i]); err != nil {
			return err
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for i := range batch {
		m := &batch[i]
		if _, err := tx.Exec(ctx, upsertMetricSQL, m.ID, m.MType, m.Value, m.Delta); err != nil {
			return fmt.Errorf("upsert metric %s: %w", m.ID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit batch: %w", err)
	}

	return nil
}

func (s *DBStorage) Get(ctx context.Context, name string, mType string) (*Metrics, error) {
	if mType != Gauge && mType != Counter {
		return nil, fmt.Errorf("unsupported metric type: %s", mType)
	}

	var value *float64
	var delta *int64

	err := s.pool.QueryRow(ctx, getMetricSQL, name, mType).Scan(&value, &delta)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%s metric %s not found", mType, name)
	}
	if err != nil {
		return nil, fmt.Errorf("query metric %s: %w", name, err)
	}

	m := &Metrics{ID: name, MType: mType}
	switch mType {
	case Gauge:
		m.Value = value
	case Counter:
		m.Delta = delta
	}

	return m, nil
}

func (s *DBStorage) GetAll(ctx context.Context) ([]Metrics, error) {
	rows, err := s.pool.Query(ctx, getAllMetricsSQL)
	if err != nil {
		return nil, fmt.Errorf("query all metrics: %w", err)
	}
	defer rows.Close()

	result := make([]Metrics, 0)
	for rows.Next() {
		var (
			key   string
			mType string
			value *float64
			delta *int64
		)
		if err := rows.Scan(&key, &mType, &value, &delta); err != nil {
			return nil, fmt.Errorf("scan metric row: %w", err)
		}

		m := Metrics{ID: key, MType: mType}
		switch mType {
		case Gauge:
			m.Value = value
		case Counter:
			m.Delta = delta
		}

		result = append(result, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate metric rows: %w", err)
	}

	return result, nil
}

func (s *DBStorage) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *DBStorage) Close() error {
	s.pool.Close()
	return nil
}
