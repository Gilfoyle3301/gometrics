package models

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

type FileStorage struct {
	memStore   *MemStorage
	file       *os.File
	mu         sync.RWMutex
	saveTicker *time.Ticker
	stopChan   chan struct{}
	logger     *zap.SugaredLogger
}

func NewFileStorage(memStore *MemStorage, filePath string, restore bool) (*FileStorage, error) {
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		slog.Error("failed to create storage directory", "error", err)
		os.Exit(1)
	}

	storageFile, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		slog.Error("failed to open file", "error", err)
		os.Exit(1)
	}
	fs := &FileStorage{
		memStore: memStore,
		file:     storageFile,
		stopChan: make(chan struct{}),
	}
	if restore {
		if err := fs.load(); err != nil {
			storageFile.Close()
			return nil, fmt.Errorf("load from file: %w", err)
		}
	}

	return fs, nil
}

func (fs *FileStorage) StartSync(interval time.Duration) {
	if interval <= 0 {
		return
	}
	fs.saveTicker = time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-fs.saveTicker.C:
				if err := fs.save(); err != nil {
					fs.logger.Errorf("failed to save metrics: %v\n", err)
				}
			case <-fs.stopChan:
				return
			}
		}
	}()
}

func (fs *FileStorage) Update(ctx context.Context, m *Metrics) error {
	return fs.memStore.Update(ctx, m)
}

func (fs *FileStorage) Get(ctx context.Context, name string, mType string) (*Metrics, error) {
	return fs.memStore.Get(ctx, name, mType)
}

func (fs *FileStorage) GetAll(ctx context.Context) ([]Metrics, error) {
	return fs.memStore.GetAll(ctx)
}

func (fs *FileStorage) Ping(ctx context.Context) error {
	return nil
}

func (fs *FileStorage) Close() error {
	if fs.saveTicker != nil {
		fs.saveTicker.Stop()
	}
	close(fs.stopChan)

	if err := fs.save(); err != nil {
		fs.file.Close()
		return fmt.Errorf("final save: %w", err)
	}

	return fs.file.Close()
}

func (fs *FileStorage) load() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	data, err := io.ReadAll(fs.file)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	if len(data) == 0 {
		return nil
	}

	var metrics []Metrics
	if err := json.Unmarshal(data, &metrics); err != nil {
		return fmt.Errorf("unmarshal metrics: %w", err)
	}

	ctx := context.Background()
	for _, m := range metrics {
		if err := fs.memStore.Update(ctx, &m); err != nil {
			return fmt.Errorf("restore metric %s: %w", m.ID, err)
		}
	}

	return nil
}

func (fs *FileStorage) save() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	ctx := context.Background()
	metrics, err := fs.memStore.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("get all metrics: %w", err)
	}

	data, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("marshal metrics: %w", err)
	}

	if err := fs.file.Truncate(0); err != nil {
		return fmt.Errorf("truncate file: %w", err)
	}

	if _, err := fs.file.Seek(0, 0); err != nil {
		return fmt.Errorf("seek file: %w", err)
	}

	if _, err := fs.file.Write(data); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	if err := fs.file.Sync(); err != nil {
		return fmt.Errorf("sync file: %w", err)
	}

	return nil
}
