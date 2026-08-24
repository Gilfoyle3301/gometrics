package agent

type Config struct {
	Address        *string `env:"ADDRESS"`
	ReportInterval *int    `env:"REPORT_INTERVAL"`
	PollInterval   *int    `env:"POLL_INTERVAL"`
}

func NewConfig() *Config {
	return &Config{}
}
