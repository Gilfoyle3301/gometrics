package server

import (
	"encoding/json"
	"fmt"
	"os"
)

// FileConfig — конфигурация сервера из JSON-файла. Поля-указатели отличают
// «опция не задана в файле» от нулевого значения: незаданные опции не
// участвуют в выборе приоритета. Имена полей соответствуют формату задания:
// address, restore, store_interval, store_file, database_dsn, crypto_key,
// а также остальные действующие опции сервера.
type FileConfig struct {
	Address           *string `json:"address"`
	Restore           *bool   `json:"restore"`
	StoreInterval     *string `json:"store_interval"`
	StoreFile         *string `json:"store_file"`
	DatabaseDSN       *string `json:"database_dsn"`
	Key               *string `json:"key"`
	CryptoKey         *string `json:"crypto_key"`
	SignatureRequired *bool   `json:"signature_required"`
	AuditFile         *string `json:"audit_file"`
	AuditURL          *string `json:"audit_url"`
}

// LoadFileConfig читает JSON-конфигурацию сервера из файла path.
// Пустой путь не является ошибкой: возвращается пустой конфиг,
// чтобы вызывающий код не проверял указатель на nil.
func LoadFileConfig(path string) (*FileConfig, error) {
	cfg := &FileConfig{}
	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	return cfg, nil
}
