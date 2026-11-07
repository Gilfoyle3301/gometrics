package agent

import (
	"encoding/json"
	"fmt"
	"os"
)

// FileConfig — конфигурация агента из JSON-файла. Поля-указатели отличают
// «опция не задана в файле» от нулевого значения. Имена полей соответствуют
// формату задания: address, report_interval, poll_interval, crypto_key,
// а также остальные действующие опции агента.
type FileConfig struct {
	Address        *string `json:"address"`
	ReportInterval *string `json:"report_interval"`
	PollInterval   *string `json:"poll_interval"`
	Key            *string `json:"key"`
	CryptoKey      *string `json:"crypto_key"`
	RateLimit      *int    `json:"rate_limit"`
}

// LoadFileConfig читает JSON-конфигурацию агента из файла path.
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
