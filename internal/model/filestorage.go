package models

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var _ Storage = (*FileStorage)(nil)

// FileStorage держит метрики в памяти и периодически сбрасывает снимок в файл.
// Доступ к метрикам наследуется от встроенного *MemStorage, поэтому здесь
// определены только файловые операции.
//
// Жизненный цикл: NewFileStorage → StartSync → Close. Close останавливает
// фоновое сохранение, дожидается его завершения и делает финальный сброс;
// повторный вызов безопасен.
type FileStorage struct {
	*MemStorage

	// fileMu сериализует файловый I/O: тиковое сохранение не должно
	// пересекаться с финальным в Close.
	fileMu    sync.Mutex
	file      *os.File
	stopChan  chan struct{}
	closeOnce sync.Once
	syncWG    sync.WaitGroup
}

// NewFileStorage открывает (при необходимости создаёт) файл хранилища.
// Все ошибки возвращаются наружу: решать, фатальны ли они, — задача вызывающего.
func NewFileStorage(memStore *MemStorage, filePath string, restore bool) (*FileStorage, error) {
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, fmt.Errorf("create storage directory: %w", err)
	}

	storageFile, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("open storage file: %w", err)
	}

	fs := &FileStorage{
		MemStorage: memStore,
		file:       storageFile,
		stopChan:   make(chan struct{}),
	}

	if restore {
		if err := fs.load(); err != nil {
			storageFile.Close()
			return nil, fmt.Errorf("load metrics: %w", err)
		}
	}

	return fs, nil
}

// StartSync запускает фоновое сохранение раз в interval. Интервал <= 0
// отключает фоновый режим — данные сохранятся только в Close. Ошибки
// сохранения передаются в onError; nil-колбэк допустим.
//
// StartSync вызывается один раз до Close.
func (fs *FileStorage) StartSync(interval time.Duration, onError func(error)) {
	if interval <= 0 {
		return
	}

	fs.syncWG.Add(1)
	go func() {
		defer fs.syncWG.Done()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				err := fs.save()
				if err != nil && onError != nil {
					onError(err)
				}
			case <-fs.stopChan:
				return
			}
		}
	}()
}

// Close останавливает фоновое сохранение и делает финальный сброс снимка.
// Идемпотентен: повторный вызов возвращает nil и не трогает закрытый файл.
func (fs *FileStorage) Close() error {
	var closeErr error

	fs.closeOnce.Do(func() {
		close(fs.stopChan)
		fs.syncWG.Wait()

		fs.fileMu.Lock()
		defer fs.fileMu.Unlock()

		if err := fs.saveLocked(); err != nil {
			fs.file.Close()
			closeErr = fmt.Errorf("final save: %w", err)
			return
		}

		closeErr = fs.file.Close()
	})

	return closeErr
}

func (fs *FileStorage) save() error {
	fs.fileMu.Lock()
	defer fs.fileMu.Unlock()

	return fs.saveLocked()
}

func (fs *FileStorage) load() error {
	data, err := io.ReadAll(fs.file)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	if len(data) == 0 {
		return nil
	}

	// Restore, а не поштучный Update: счётчики восстанавливаются присваиванием
	// сохранённого значения, иначе повторное чтение файла накрутило бы их дважды.
	return fs.MemStorage.Restore(data)
}

// saveLocked перезаписывает файл текущим снимком метрик. Вызывается под fileMu.
func (fs *FileStorage) saveLocked() error {
	metrics, err := fs.MemStorage.GetAll(context.Background())
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
