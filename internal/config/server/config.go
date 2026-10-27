// Package server содержит конфигурацию сервера метрик,
// заполняемую из переменных окружения.
package server

// Config — конфигурация сервера, заполняемая из переменных окружения.
// Поля-указатели отличают «переменная не задана» от пустого значения:
// для незаданных полей используются значения флагов командной строки.
type Config struct {
	Address           *string `env:"ADDRESS"`
	StoreInterval     *int    `env:"STORE_INTERVAL"`
	FileStoragePath   *string `env:"FILE_STORAGE_PATH"`
	Restore           *bool   `env:"RESTORE"`
	DatabaseDSN       *string `env:"DATABASE_DSN"`
	Key               *string `env:"KEY"`
	SignatureRequired *bool   `env:"SIGNATURE_REQUIRED"`
	AuditFile         *string `env:"AUDIT_FILE"`
	AuditURL          *string `env:"AUDIT_URL"`
	CryptoKey         *string `env:"CRYPTO_KEY"`
	Config            *string `env:"CONFIG"`
}

// NewConfig создаёт пустой конфиг для последующего разбора переменных окружения.
func NewConfig() *Config {
	return &Config{}
}
