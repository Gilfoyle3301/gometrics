package audit

import (
	"encoding/json"
	"fmt"
	"os"
)

// FileObserver — приёмник аудита, дописывающий события в конец файла
// по одной JSON-строке на событие.
type FileObserver struct {
	file *os.File
}

// NewFileObserver открывает файл path для дозаписи, создавая его при
// необходимости.
func NewFileObserver(path string) (*FileObserver, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open audit file: %w", err)
	}
	return &FileObserver{file: f}, nil
}

// Update дописывает событие в файл одной JSON-строкой с переводом строки.
func (o *FileObserver) Update(e Event) error {
	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal audit event: %w", err)
	}

	if _, err := o.file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write audit event: %w", err)
	}
	return nil
}

// Close закрывает файл аудита.
func (o *FileObserver) Close() error {
	return o.file.Close()
}
