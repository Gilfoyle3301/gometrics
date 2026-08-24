package agent

import "time"

type Config struct {
	Address        *string        `env:"ADDRESS"`
	ReportInterval *time.Duration `env:"REPORT_INTERVAL"`
	PollInterval   *time.Duration `env:"POLL_INTERVAL"`
}

func NewConfig() *Config {
	return &Config{}
}
