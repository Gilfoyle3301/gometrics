package server

type Config struct {
	Address           *string `env:"ADDRESS"`
	StoreInterval     *int    `env:"STORE_INTERVAL"`
	FileStoragePath   *string `env:"FILE_STORAGE_PATH"`
	Restore           *bool   `env:"RESTORE"`
	DatabaseDSN       *string `env:"DATABASE_DSN"`
	Key               *string `env:"KEY"`
	SignatureRequired *bool   `env:"SIGNATURE_REQUIRED"`
}

func NewConfig() *Config {
	return &Config{}
}
