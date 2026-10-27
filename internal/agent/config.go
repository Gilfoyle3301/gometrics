package agent

import "strings"

// Config — конфигурация агента, заполняемая из переменных окружения.
// Поля-указатели отличают «переменная не задана» от пустого значения:
// для незаданных полей используются значения флагов командной строки.
type Config struct {
	Address        *string `env:"ADDRESS"`
	ReportInterval *int    `env:"REPORT_INTERVAL"`
	PollInterval   *int    `env:"POLL_INTERVAL"`
	Key            *string `env:"KEY"`
	RateLimit      *int    `env:"RATE_LIMIT"`
}

// NewConfig создаёт пустой конфиг агента для разбора переменных окружения.
func NewConfig() *Config {
	return &Config{}
}

// NormalizeAddress добавляет к адресу схему http://, если схема не указана.
func NormalizeAddress(addr string) string {
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return addr
	}
	return "http://" + addr
}
