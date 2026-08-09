package server

import "time"

type Config struct {
	Address         *string        `env:"ADDRESS"`
	StoreInterval   *time.Duration `env:"STORE_INTERVAL"`
	FileStoragePath *string        `env:"FILE_STORAGE_PATH"`
	Restore         *bool          `env:"RESTORE"`
	DatabaseDSN     *string        `env:"DATABASE_DSN"`
}

func NewConfig() *Config {
	return &Config{}
}
