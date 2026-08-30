package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfig(t *testing.T) {
	cfg := NewConfig()

	require.NotNil(t, cfg)
	assert.Nil(t, cfg.Address)
	assert.Nil(t, cfg.ReportInterval)
	assert.Nil(t, cfg.PollInterval)
}

func TestNormalizeAddress(t *testing.T) {
	tests := []struct {
		addr string
		want string
	}{
		{"localhost:8080", "http://localhost:8080"},
		{"http://localhost:8080", "http://localhost:8080"},
		{"https://metrics.example.com", "https://metrics.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			assert.Equal(t, tt.want, NormalizeAddress(tt.addr))
		})
	}
}
